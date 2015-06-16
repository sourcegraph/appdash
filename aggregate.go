package appdash

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

func init() {
	RegisterEvent(AggregateEvent{})
}

// AggregateEvent represents an aggregated set of timespan events. This is the
// only type of event produced by the AggregateStore type.
type AggregateEvent struct {
	// The root span name of every item in this aggregated set of timespan events.
	Name string

	// Every root span start and end time, from which other information can be
	// calculated.
	Times [][2]time.Time

	// Trace IDs for the slowest of the above times (useful for inspection).
	Slowest []ID
}

// Schema implements the Event interface.
func (e AggregateEvent) Schema() string { return "aggregate" }

// TODO(slimsag): do not encode aggregate events in JSON. We have to do this for
// now because the reflection code can't handle *Trace types sufficiently.

// MarshalEvent implements the EventMarshaler interface.
func (e AggregateEvent) MarshalEvent() (Annotations, error) {
	// Encode the entire event as JSON.
	data, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return Annotations{
		{Key: "JSON", Value: data},
	}, nil
}

// UnmarshalEvent implements the EventUnmarshaler interface.
func (e AggregateEvent) UnmarshalEvent(as Annotations) (Event, error) {
	// Find the annotation with our key.
	for _, ann := range as {
		if ann.Key != "JSON" {
			continue
		}
		err := json.Unmarshal(ann.Value, &e)
		if err != nil {
			return nil, fmt.Errorf("AggregateEvent.UnmarshalEvent: %v", err)
		}
		return e, nil
	}
	return nil, errors.New("expected one annotation with key=\"JSON\"")
}

// spanGroupSlowest represents one of the slowest traces in a span group.
type spanGroupSlowest struct {
	TraceID    ID        // Root span ID of the slowest trace.
	Start, End time.Time // Start and end time of the slowest trace.
}

// empty tells if this spanGroupSlowest slot is empty / uninitialized.
func (s spanGroupSlowest) empty() bool {
	return s == spanGroupSlowest{}
}

// spanGroup represents all of the times for the root spans (i.e. traces) of the
// given name. It also contains the N-slowest traces of the group.
type spanGroup struct {
	// Trace is the trace ID that the generated AggregateEvent has been placed
	// into for collection.
	Trace   ID
	Name    string             // Root span name (e.g. the route for httptrace).
	Times   [][2]time.Time     // Aggregated timespans for the traces.
	Slowest []spanGroupSlowest // N-slowest traces in the group.
}

func (s spanGroup) Len() int      { return len(s.Slowest) }
func (s spanGroup) Swap(i, j int) { s.Slowest[i], s.Slowest[j] = s.Slowest[j], s.Slowest[i] }
func (s spanGroup) Less(i, j int) bool {
	a := s.Slowest[i]
	b := s.Slowest[j]
	return a.End.Sub(a.Start) > b.End.Sub(b.Start)
}

// update updates the span group to account for a potentially slowest trace,
// returning whether or not the given trace was indeed slowest.
func (s *spanGroup) update(start, end time.Time, trace ID, remove func(trace ID)) bool {
	s.Times = append(s.Times, [2]time.Time{start, end})

	// The s.Slowest list is kept sorted from slowest to fastest. As we want to
	// steal the slot from the fastest (or zero) one we iterate over it
	// backwards comparing times.
	for i := len(s.Slowest) - 1; i > 0; i-- {
		sm := s.Slowest[i]
		if sm.TraceID == trace {
			// Trace is already inside the group as one of the slowest.
			return false
		}

		// If our time is lesser than the trace in the slot already, we aren't
		// slower so don't steal the slot.
		if end.Sub(start) < sm.End.Sub(sm.Start) {
			continue
		}

		// If there is already a trace inside this group (i.e. we are taking its
		// spot as one of the slowest), then we must request for its removal from
		// the output store.
		if sm.TraceID != 0 {
			remove(sm.TraceID)
		}

		s.Slowest[i] = spanGroupSlowest{
			TraceID: trace,
			Start:   start,
			End:     end,
		}
		sort.Sort(s)
		return true
	}
	return false
}

// evictBefore evicts all times in the group
func (s *spanGroup) evictBefore(tnano int64, debug bool) {
	count := 0
search:
	for i, ts := range s.Times {
		if ts[0].UnixNano() < tnano {
			s.Times = append(s.Times[:i], s.Times[i+1:]...)
			count++
			goto search
		}
	}

	if debug && count > 0 {
		log.Printf("AggregateStore: evicted %d timespans from the group %q", count, s.Name)
	}
}

// AggregateStore aggregates timespan events into groups based on the root span
// name. Much like a RecentStore, it evicts aggregated events after a certain
// time period.
type AggregateStore struct {
	// MinEvictAge is the minimum age of group data before it is evicted.
	MinEvictAge time.Duration

	// MaxRate is the maximum expected rate of incoming traces.
	//
	// Multiple traces can be collected at once, and they must be queued up in
	// memory until the entire trace has been collected, otherwise the N-slowest
	// traces cannot be stored.
	//
	// If the number is too large, a lot of memory will be used (to store
	// MaxRate number of traces), and if too small some then some aggregate
	// events will not have the full N-slowest traces associated with them.
	MaxRate int

	// NSlowest is the number of slowest traces to fully keep for inspection for
	// each group.
	NSlowest int

	// Debug is whether or not to log debug messages.
	Debug bool

	// MemoryStore is the memory store were aggregated traces are saved to and
	// deleted from. It is the final destination for traces.
	*MemoryStore

	// Keep is a store that is sent each collection directly if non-nil. Once a
	// trace would be evicted from the MemoryStore (i.e. if it's no longer one
	// of the N-slowest traces for a group), this store is queried for the
	// trace. If the trace exists the trace is kept in the memory store,
	// otherwise it is deleted.
	//
	// This field is useful for saying "keep the N slowest traces AND all traces
	// in the past 15 minutes (by setting Keep == RecentStore)", likewise it can
	// be used with LimitStore, etc.
	Keep Store

	mu           sync.Mutex
	groups       map[ID]*spanGroup // map of trace ID to span group.
	groupsByName map[string]ID     // looks up a groups trace ID by name.
	pre          *LimitStore       // traces which do not have span groups yet
	lastEvicted  time.Time         // last time that eviction ran
}

// NewAggregateStore is short-hand for:
//
//  store := &AggregateStore{
//      MinEvictAge: 72 * time.Hour,
//      MaxRate: 4096,
//      NSlowest: 5,
//      MemoryStore: NewMemoryStore(),
//  }
//
func NewAggregateStore() *AggregateStore {
	return &AggregateStore{
		MinEvictAge: 72 * time.Hour,
		MaxRate:     4096,
		NSlowest:    5,
		MemoryStore: NewMemoryStore(),
	}
}

// Collect calls the underlying store's Collect, deleting the oldest
// trace if the capacity has been reached.
func (as *AggregateStore) Collect(id SpanID, anns ...Annotation) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	// Initialization
	if as.groups == nil {
		as.groups = make(map[ID]*spanGroup)
		as.groupsByName = make(map[string]ID)
		as.pre = &LimitStore{
			Max:         as.MaxRate,
			DeleteStore: NewMemoryStore(),
		}
	}

	// Collect into the limit store.
	if err := as.pre.Collect(id, anns...); err != nil {
		return err
	}

	// Consider eviction of old data.
	if time.Since(as.lastEvicted) > as.MinEvictAge {
		if err := as.evictBefore(time.Now().Add(-1 * as.MinEvictAge)); err != nil {
			return err
		}
	}

	// Grab the group for our span.
	group, ok := as.group(id, anns...)
	if !ok {
		// We don't have a group for the trace, and can't create one (the
		// spanName event isn't present yet).
		return nil
	}

	// Unmarshal the events.
	var events []Event
	if err := UnmarshalEvents(anns, &events); err != nil {
		return err
	}

	// Find the start and end time of the trace.
	eStart, eEnd, ok := findTraceTimes(events)
	if !ok {
		// We didn't find any timespan events at all, so we're done here.
		return nil
	}

	// Update the group to consider this trace being one of the slowest.
	group.update(eStart, eEnd, id.Trace, func(trace ID) {
		// Delete the request trace from the output store.
		if err := as.deleteOutput(trace); err != nil {
			log.Printf("AggregateStore: failed to delete a trace: %s", err)
		}
	})

	// Move traces from the limit store into the group, as needed.
	for _, slowest := range group.Slowest {
		// Find the trace in the limit store.
		trace, err := as.pre.Trace(slowest.TraceID)
		if err == ErrTraceNotFound {
			continue
		}
		if err != nil {
			return err
		}

		// Place into output store.
		var walk func(t *Trace) error
		walk = func(t *Trace) error {
			err := as.MemoryStore.Collect(t.Span.ID, t.Span.Annotations...)
			if err != nil {
				return err
			}
			for _, sub := range t.Sub {
				if err := walk(sub); err != nil {
					return err
				}
			}
			return nil
		}
		if err := walk(trace); err != nil {
			return err
		}

		// Delete from the limit store.
		err = as.pre.Delete(slowest.TraceID)
		if err != nil {
			return err
		}
	}

	// Prepare the aggregation event (before locking below).
	ev := &AggregateEvent{
		Name:  group.Name,
		Times: group.Times,
	}
	for _, slowest := range group.Slowest {
		if !slowest.empty() {
			ev.Slowest = append(ev.Slowest, slowest.TraceID)
		}
	}
	if as.Debug && len(ev.Slowest) == 0 {
		log.Printf("AggregateStore: no slowest traces for group %q (consider increasing MaxRate)", group.Name)
	}

	// As we're updating the aggregation event, we go ahead and delete the old
	// one now. We do this all under as.MemoryStore.Lock otherwise users (e.g. the
	// web UI) can pull from as.MemoryStore when the trace has been deleted.
	as.MemoryStore.Lock()
	defer as.MemoryStore.Unlock()
	if err := as.MemoryStore.deleteNoLock(group.Trace); err != nil {
		return err
	}

	// Record an aggregate event with the given name.
	recEvent := func(e Event) error {
		anns, err := MarshalEvent(e)
		if err != nil {
			return err
		}
		return as.MemoryStore.collectNoLock(SpanID{Trace: group.Trace}, anns...)
	}
	if err := recEvent(spanName{Name: group.Name}); err != nil {
		return err
	}
	if err := recEvent(ev); err != nil {
		return err
	}
	return nil
}

// deleteOutput deletes the given traces from the output memory store. If
// as.Keep is not nil and it has a trace still, it is kept rather than deleted.
func (as *AggregateStore) deleteOutput(traces ...ID) error {
	for _, trace := range traces {
		if as.Keep != nil {
			// Find the trace in the keep store.
			_, err := as.Keep.Trace(trace)
			if err == nil {
				// We found it, and so we keep the trace.
				return nil
			} else if err != ErrTraceNotFound {
				return err
			}
		}
		if err := as.MemoryStore.Delete(trace); err != nil {
			return err
		}
	}
	return nil
}

// group returns the span group that the collection belongs in, or nil, false if
// no such span group exists / could be created.
//
// The as.mu lock must be held for this method to operate safely.
func (as *AggregateStore) group(id SpanID, anns ...Annotation) (*spanGroup, bool) {
	// Do nothing if we already have a group associated with our root span.
	if group, ok := as.groups[id.Trace]; ok {
		return group, true
	}

	// At this point, we need a root span or else we can't create the group.
	if !id.IsRoot() {
		return nil, false
	}

	// And likewise, always a name event.
	var name spanName
	if err := UnmarshalEvent(anns, &name); err != nil {
		return nil, false
	}

	// If there already exists a group with that name, then we just associate
	// our trace with that group for future lookup and we're good to go.
	if groupID, ok := as.groupsByName[name.Name]; ok {
		group := as.groups[groupID]
		as.groups[id.Trace] = group
		return group, true
	}

	// Create a new group, and associate our trace with it.
	group := &spanGroup{
		Name:    name.Name,
		Trace:   NewRootSpanID().Trace, //id.Trace,
		Slowest: make([]spanGroupSlowest, as.NSlowest),
	}
	as.groups[id.Trace] = group
	as.groupsByName[name.Name] = id.Trace
	return group, true
}

// evictBefore evicts aggregation events that were created before t.
//
// The as.mu lock must be held for this method to operate safely.
func (as *AggregateStore) evictBefore(t time.Time) error {
	evictStart := time.Now()
	as.lastEvicted = evictStart
	tnano := t.UnixNano()

	// Build a list of aggregation events to evict.
	var toEvict []ID
	for _, group := range as.groups {
		group.evictBefore(tnano, as.Debug)

	searchSlowest:
		for i, sm := range group.Slowest {
			if !sm.empty() && sm.Start.UnixNano() < tnano {
				group.Slowest = append(group.Slowest[:i], group.Slowest[i+1:]...)
				toEvict = append(toEvict, sm.TraceID)
				goto searchSlowest
			}
		}

		// If the group is not complete empty, we have nothing more to do.
		if len(group.Times) > 0 || len(group.Slowest) > 0 {
			continue
		}

		// Remove the empty group from the maps, noting that as.groups often
		// has multiple references to the same group.
		for id, g := range as.groups {
			if g == group {
				delete(as.groups, id)
			}
		}
		delete(as.groupsByName, group.Name)

		// Also request removal of the group (AggregateEvent) from the output store.
		err := as.deleteOutput(group.Trace)
		if err != nil {
			return err
		}
	}

	// We are done if there is nothing to evict.
	if len(toEvict) == 0 {
		return nil
	}

	if as.Debug {
		log.Printf("AggregateStore: deleting %d slowest traces created before %s (age check took %s)", len(toEvict), t, time.Since(evictStart))
	}

	// Spawn separate goroutine so we don't hold the as.mu lock.
	go func() {
		deleteStart := time.Now()
		if err := as.deleteOutput(toEvict...); err != nil {
			log.Printf("AggregateStore: failed to delete slowest traces: %s", err)
		}
		if as.Debug {
			log.Printf("AggregateStore: finished deleting %d slowest traces created before %s (took %s)", len(toEvict), t, time.Since(deleteStart))
		}
	}()
	return nil
}

// findTraceTimes finds the minimum and maximum timespan event times for the
// given set of events, or returns ok == false if there are no such events.
func findTraceTimes(events []Event) (start, end time.Time, ok bool) {
	// Find the start and end time of the trace.
	var (
		eStart, eEnd time.Time
		haveTimes    = false
	)
	for _, e := range events {
		e, ok := e.(TimespanEvent)
		if !ok {
			continue
		}
		if !haveTimes {
			haveTimes = true
			eStart = e.Start()
			eEnd = e.End()
			continue
		}
		if v := e.Start(); v.UnixNano() < eStart.UnixNano() {
			eStart = v
		}
		if v := e.End(); v.UnixNano() > eEnd.UnixNano() {
			eEnd = v
		}
	}
	if !haveTimes {
		// We didn't find any timespan events at all, so we're done here.
		ok = false
		return
	}
	return eStart, eEnd, true
}
