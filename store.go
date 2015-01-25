package apptrace

import (
	"encoding/gob"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"
)

// A Store stores and retrieves spans.
type Store interface {
	Collector

	// Trace gets a trace (a tree of spans) given its trace ID. If no
	// such trace exists, ErrTraceNotFound is returned.
	Trace(ID) (*Trace, error)
}

var (
	// ErrTraceNotFound is returned by Store.GetTrace when no trace is
	// found with the given ID.
	ErrTraceNotFound = errors.New("trace not found")
)

// A Queryer indexes spans and makes them queryable.
type Queryer interface {
	// Traces returns an implementation-defined list of traces. It is
	// a placeholder method that will be removed when other, more
	// useful methods are added to Queryer.
	Traces() ([]*Trace, error)
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		trace: map[ID]*Trace{},
		span:  map[ID]map[ID]*Trace{},
	}
}

// A MemoryStore is an in-memory Store.
type MemoryStore struct {
	trace map[ID]*Trace        // trace ID -> trace tree
	span  map[ID]map[ID]*Trace // trace ID -> span ID -> trace (sub)tree

	sync.Mutex // protects trace

	log bool
}

// Compile-time "implements" check.
var _ interface {
	Store
	Queryer
} = (*MemoryStore)(nil)

// Collect implements the Collector interface by collecting the events that
// occured in the span in-memory.
func (ms *MemoryStore) Collect(id SpanID, as ...Annotation) error {
	ms.Lock()
	defer ms.Unlock()

	if ms.log {
		log.Printf("Collect %v", id)
	}

	// Initialize span map if needed.
	if _, present := ms.span[id.Trace]; !present {
		ms.span[id.Trace] = map[ID]*Trace{}
	}

	// Create or update span.
	s, present := ms.span[id.Trace][id.Span]
	if !present {
		s = &Trace{Span: Span{ID: id, Annotations: as}}
		ms.span[id.Trace][id.Span] = s
	} else {
		if ms.log {
			if len(as) > 0 {
				log.Printf("Add %d annotations to %v", len(as), id)
			}
		}
		s.Annotations = append(s.Annotations, as...)
		return nil
	}

	// Create trace tree if it doesn't already exist.
	root, present := ms.trace[id.Trace]
	if !present {
		// Root span hasn't been seen yet, so make this the temporary
		// root (until we collect the actual root).
		if ms.log {
			if id.IsRoot() {
				log.Printf("Create trace %v root %v", id.Trace, id)
			} else {
				log.Printf("Create temporary trace %v root %v", id.Trace, id)
			}
		}
		ms.trace[id.Trace] = s
		root = s
	}

	// If there's a temp root and we just collected the real
	// root, fix up the tree. Or if we're the temp root's
	// parents, set us up as the new temp root.
	if isRoot, isTempRootParent := id.IsRoot(), root.Span.ID.Parent == id.Span; s != root && (isRoot || isTempRootParent) {
		oldRoot := root
		root = s
		if ms.log {
			if isRoot {
				log.Printf("Set real root %v and move temp root %v", root.Span.ID, oldRoot.Span.ID)
			} else {
				log.Printf("Set new temp root %v and move previous temp root %v (child of new temp root)", root.Span.ID, oldRoot.Span.ID)
			}
		}
		ms.trace[id.Trace] = root // set new root
		ms.reattachChildren(root, oldRoot)
		ms.insert(root, oldRoot) // reinsert the old root

		// Move the old temp root's temp children to the new
		// (possibly temp) root.
		var sub2 []*Trace
		for _, c := range oldRoot.Sub {
			if c.Span.ID.Parent != oldRoot.Span.ID.Span {
				if ms.log {
					log.Printf("Move %v from old root %v to new (possibly temp) root %v", c.Span.ID, oldRoot.Span.ID, root.Span.ID)
				}
				root.Sub = append(root.Sub, c)
			} else {
				sub2 = append(sub2, c)
			}
		}
		oldRoot.Sub = sub2
	}

	// Insert into trace tree. (We inserted the trace root span
	// above.)
	if !id.IsRoot() && s != root {
		ms.insert(root, s)
	}

	// See if we're the parent of any of the root's temporary
	// children.
	if s != root {
		ms.reattachChildren(s, root)
	}

	return nil
}

// insert inserts t into the trace tree whose root (or temp root) is
// root.
func (ms *MemoryStore) insert(root, t *Trace) {
	p, present := ms.span[t.ID.Trace][t.ID.Parent]
	if present {
		if ms.log {
			log.Printf("Add %v as a child of parent %v", t.Span.ID, p.Span.ID)
		}
		p.Sub = append(p.Sub, t)
	} else {
		// Add as temporary child of the root for now. When the
		// real parent is added, we'll fix it up later.
		if ms.log {
			log.Printf("Add %v as a temporary child of root %v", t.Span.ID, root.Span.ID)
		}
		root.Sub = append(root.Sub, t)
	}
}

// reattachChildren moves temporary children of src to dst, if dst is
// the node's parent.
func (ms *MemoryStore) reattachChildren(dst, src *Trace) {
	if dst == src {
		panic("dst == src")
	}
	var sub2 []*Trace
	for _, c := range src.Sub {
		if c.Span.ID.Parent == dst.Span.ID.Span {
			if ms.log {
				log.Printf("Move %v from src %v to dst %v", c.Span.ID, src.Span.ID, dst.Span.ID)
			}
			dst.Sub = append(dst.Sub, c)
		} else {
			sub2 = append(sub2, c)
		}
	}
	src.Sub = sub2
}

// Trace implements the Store interface by returning the Trace (a tree of
// spans) for the given trace span ID or, if no such trace exists, by returning
// ErrTraceNotFound.
func (ms *MemoryStore) Trace(id ID) (*Trace, error) {
	ms.Lock()
	defer ms.Unlock()

	return ms.traceNoLock(id)
}

func (ms *MemoryStore) traceNoLock(id ID) (*Trace, error) {
	t, present := ms.trace[id]
	if !present {
		return nil, ErrTraceNotFound
	}
	return t, nil
}

// Traces implements the Queryer interface.
func (ms *MemoryStore) Traces() ([]*Trace, error) {
	ms.Lock()
	defer ms.Unlock()

	var ts []*Trace
	for id := range ms.trace {
		t, err := ms.traceNoLock(id)
		if err != nil {
			return nil, err
		}
		ts = append(ts, t)
	}
	return ts, nil
}

// Delete implements the DeleteStore interface by deleting the traces given by
// their span ID's from this in-memory store.
func (ms *MemoryStore) Delete(traces ...ID) error {
	ms.Lock()
	defer ms.Unlock()

	for _, id := range traces {
		delete(ms.trace, id)
		delete(ms.span, id)
	}
	return nil
}

type memoryStoreData struct {
	Trace map[ID]*Trace
	Span  map[ID]map[ID]*Trace
}

// Write writes ms's internal data structures.
func (ms *MemoryStore) Write(w io.Writer) error {
	ms.Lock()
	defer ms.Unlock()

	data := memoryStoreData{ms.trace, ms.span}
	return gob.NewEncoder(w).Encode(data)
}

// ReadFrom loads ms's internal data structures from a reader.
func (ms *MemoryStore) ReadFrom(r io.Reader) (int64, error) {
	ms.Lock()
	defer ms.Unlock()

	var data memoryStoreData
	if err := gob.NewDecoder(r).Decode(&data); err != nil {
		return 0, err
	}
	ms.trace = data.Trace
	ms.span = data.Span
	return int64(len(ms.trace)), nil
}

// PersistentStore is a Store that can persist its data and read it
// back in.
type PersistentStore interface {
	Write(io.Writer) error
	ReadFrom(io.Reader) (int64, error)
	Store
}

// PersistEvery persists s's data to a file periodically.
func PersistEvery(s PersistentStore, interval time.Duration, file string) error {
	for {
		time.Sleep(interval)

		f, err := ioutil.TempFile("", "apptrace")
		if err != nil {
			return err
		}
		if err := s.Write(f); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		if err := os.Rename(f.Name(), file); err != nil {
			return err
		}
	}
}

// A DeleteStore is a Store that can delete traces.
type DeleteStore interface {
	Store

	// Delete deletes traces given their trace IDs.
	Delete(...ID) error
}

// A RecentStore wraps another store and deletes old traces after a
// specified amount of time.
type RecentStore struct {
	// MinEvictAge is the minimum age of a trace before it is evicted.
	MinEvictAge time.Duration

	// DeleteStore is the underlying store that spans are saved to and
	// deleted from.
	DeleteStore

	// Debug is whether to log debug messages.
	Debug bool

	// created maps trace ID to the UnixNano time it was first seen.
	created map[ID]int64

	// lastEvicted is the last time the eviction process was run.
	lastEvicted time.Time

	mu sync.Mutex // mu guards created and lastEvicted
}

// Collect calls the underlying store's Collect and records the time
// that this trace was first seen.
func (rs *RecentStore) Collect(id SpanID, anns ...Annotation) error {
	rs.mu.Lock()
	if rs.created == nil {
		rs.created = map[ID]int64{}
	}
	if _, present := rs.created[id.Trace]; !present {
		rs.created[id.Trace] = time.Now().UnixNano()
	}
	if time.Since(rs.lastEvicted) > rs.MinEvictAge {
		rs.evictBefore(time.Now().Add(-1 * rs.MinEvictAge))
	}
	rs.mu.Unlock()

	return rs.DeleteStore.Collect(id, anns...)
}

// evictBefore evicts traces that were created before t. The rs.mu lock
// must be held while calling evictBefore.
func (rs *RecentStore) evictBefore(t time.Time) {
	evictStart := time.Now()
	tnano := t.UnixNano()
	var toEvict []ID
	for id, ct := range rs.created {
		if ct < tnano {
			toEvict = append(toEvict, id)
			delete(rs.created, id)
		}
	}
	if len(toEvict) == 0 {
		return
	}

	if rs.Debug {
		log.Printf("RecentStore: deleting %d traces created before %s (age check took %s)", len(toEvict), t, time.Since(evictStart))
	}

	// Spawn separate goroutine so we don't hold the rs.mu lock.
	go func() {
		deleteStart := time.Now()
		if err := rs.DeleteStore.Delete(toEvict...); err != nil {
			log.Printf("RecentStore: failed to delete traces: %s", err)
		}
		if rs.Debug {
			log.Printf("RecentStore: finished deleting %d traces created before %s (took %s)", len(toEvict), t, time.Since(deleteStart))
		}
	}()
}
