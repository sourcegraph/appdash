package appdash

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestMemoryStore_Collect_notFound(t *testing.T) {
	ms := storeT{t, NewMemoryStore()}

	if x, err := ms.Trace(123); err != ErrTraceNotFound {
		t.Errorf("Trace(123): got trace %+v and err %#v, want ErrTraceNotFound", x, err)
	}
}

func TestMemoryStore_Collect_one(t *testing.T) {
	ms := storeT{t, NewMemoryStore()}

	t.Log("collect trace 1")
	ms.MustCollect(SpanID{1, 1, 0})
	want1 := &Trace{Span: Span{ID: SpanID{1, 1, 0}}}
	if x := ms.MustTrace(1); !reflect.DeepEqual(x, want1) {
		t.Errorf("Trace(1): got trace %+v, want %+v", x, want1)
	}
}

func TestMemoryStore_Collect_collectSameTwice(t *testing.T) {
	ms := storeT{t, NewMemoryStore()}

	t.Log("collect trace 1")
	ms.MustCollect(SpanID{1, 1, 0})

	t.Log("collect trace 1 again")
	ms.MustCollect(SpanID{1, 1, 0})
	want1 := &Trace{Span: Span{ID: SpanID{1, 1, 0}}}
	if x := ms.MustTrace(1); !reflect.DeepEqual(x, want1) {
		t.Errorf("Trace(1): got trace %+v, want %+v", x, want1)
	}
}

func TestMemoryStore_Collect_collectSameChildTwice(t *testing.T) {
	ms := storeT{t, NewMemoryStore()}

	t.Log("collect trace 1")
	ms.MustCollect(SpanID{1, 1, 0})

	t.Log("collect trace 2")
	ms.MustCollect(SpanID{1, 2, 1}, Annotation{Key: "k1"})
	ms.MustCollect(SpanID{1, 2, 1}, Annotation{Key: "k2"})
	want1 := &Trace{
		Span: Span{ID: SpanID{1, 1, 0}},
		Sub: []*Trace{
			{Span: Span{SpanID{1, 2, 1}, Annotations{{Key: "k1"}, {Key: "k2"}}}},
		},
	}
	if x := ms.MustTrace(1); !reflect.DeepEqual(x, want1) {
		t.Errorf("Trace(1): got trace %+v, want %+v", x, want1)
	}
}

func TestMemoryStore_Collect_collectTwo(t *testing.T) {
	ms := storeT{t, NewMemoryStore()}

	t.Log("collect trace 1")
	ms.MustCollect(SpanID{1, 1, 0})

	t.Log("collect trace 2")
	ms.MustCollect(SpanID{2, 1, 0})
	want2 := &Trace{Span: Span{ID: SpanID{2, 1, 0}}}
	if x := ms.MustTrace(2); !reflect.DeepEqual(x, want2) {
		t.Errorf("Trace(2): got trace %+v, want %+v", x, want2)
	}

	want1 := &Trace{Span: Span{ID: SpanID{1, 1, 0}}}
	if x := ms.MustTrace(1); !reflect.DeepEqual(x, want1) {
		t.Errorf("Trace(1): got trace %+v, want %+v", x, want1)
	}
	if x := ms.MustTrace(2); !reflect.DeepEqual(x, want2) {
		t.Errorf("Trace(2): got trace %+v, want %+v", x, want2)
	}
}

func TestMemoryStore_Collect_oneChild(t *testing.T) {
	ms := storeT{t, NewMemoryStore()}

	t.Log("collect trace 1")
	ms.MustCollect(SpanID{1, 1, 0})

	t.Log("collect trace 1 child")
	ms.MustCollect(SpanID{1, 2, 1})

	want1 := &Trace{
		Span: Span{ID: SpanID{1, 1, 0}},
		Sub: []*Trace{
			{
				Span: Span{ID: SpanID{1, 2, 1}},
			},
		},
	}
	if x := ms.MustTrace(1); !reflect.DeepEqual(x, want1) {
		t.Errorf("Trace(1): got trace %+v, want %+v", x, want1)
	}
}

func TestMemoryStore_deleteSubNoLock(t *testing.T) {
	s := NewMemoryStore()
	ms := storeT{t, s}

	// Collect trace / root span.
	ms.MustCollect(SpanID{1, 1, 0})

	// Collect child span.
	childSpanID := SpanID{1, 2, 1}
	ms.MustCollect(childSpanID)

	// Validate that removal of the child span functions properly.
	s.Lock()
	if !s.deleteSubNoLock(childSpanID, false) {
		t.Fatal("failed to delete subspan")
	}
	s.Unlock()

	want1 := &Trace{
		Span: Span{ID: SpanID{1, 1, 0}},
		Sub:  []*Trace{},
	}
	if x := ms.MustTrace(1); !reflect.DeepEqual(x, want1) {
		t.Errorf("Trace(1): got trace %+v, want %+v", x, want1)
	}
}

func TestMemoryStore_Collect_childCollectedBeforeRoot(t *testing.T) {
	ms := storeT{t, NewMemoryStore()}

	t.Log("collect trace 1 child")
	ms.MustCollect(SpanID{1, 2, 1})
	want1 := &Trace{Span: Span{ID: SpanID{1, 2, 1}}}
	if x := ms.MustTrace(1); !reflect.DeepEqual(x, want1) {
		t.Errorf("Trace(1): got trace %+v, want %+v", x, want1)
	}

	t.Log("collect trace 1 root")
	ms.MustCollect(SpanID{1, 1, 0})

	want1 = &Trace{
		Span: Span{ID: SpanID{1, 1, 0}},
		Sub: []*Trace{
			{
				Span: Span{ID: SpanID{1, 2, 1}},
			},
		},
	}
	if x := ms.MustTrace(1); !reflect.DeepEqual(x, want1) {
		t.Errorf("Trace(1): got trace %+v, want %+v", x, want1)
	}
}

func TestMemoryStore_Collect_childrenCollectedInReverse(t *testing.T) {
	ms := storeT{t, NewMemoryStore()}

	t.Log("collect trace 1 child 4")
	ms.MustCollect(SpanID{1, 4, 3})
	want4 := &Trace{Span: Span{ID: SpanID{1, 4, 3}}}
	if x := ms.MustTrace(1); !reflect.DeepEqual(x, want4) {
		t.Errorf("Trace(1): got trace %+v, want %+v", x, want4)
	}

	t.Log("collect trace 1 child 3")
	ms.MustCollect(SpanID{1, 3, 2})
	want3 := &Trace{
		Span: Span{ID: SpanID{1, 3, 2}},
		Sub: []*Trace{
			{
				Span: Span{ID: SpanID{1, 4, 3}},
			},
		},
	}
	if x := ms.MustTrace(1); !reflect.DeepEqual(x, want3) {
		t.Errorf("Trace(1): got trace %+v, want %+v", x, want3)
	}

	t.Log("collect trace 1 child 2")
	ms.MustCollect(SpanID{1, 2, 1})
	want2 := &Trace{
		Span: Span{ID: SpanID{1, 2, 1}},
		Sub: []*Trace{
			{
				Span: Span{ID: SpanID{1, 3, 2}},
				Sub: []*Trace{
					{
						Span: Span{ID: SpanID{1, 4, 3}},
					},
				},
			},
		},
	}
	if x := ms.MustTrace(1); !reflect.DeepEqual(x, want2) {
		t.Errorf("Trace(1): got trace %+v, want %+v", x, want2)
	}

	t.Log("collect trace 1 root")
	ms.MustCollect(SpanID{1, 1, 0})

	want1 := &Trace{
		Span: Span{ID: SpanID{1, 1, 0}},
		Sub: []*Trace{
			{
				Span: Span{ID: SpanID{1, 2, 1}},
				Sub: []*Trace{
					{
						Span: Span{ID: SpanID{1, 3, 2}},
						Sub: []*Trace{
							{
								Span: Span{ID: SpanID{1, 4, 3}},
							},
						},
					},
				},
			},
		},
	}
	if x := ms.MustTrace(1); !reflect.DeepEqual(x, want1) {
		t.Errorf("Trace(1): got trace %+v, want %+v", x, want1)
	}
}

func TestMemoryStore_Collect_fuzz(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ms := storeT{t, NewMemoryStore()}

	const n = 2000
	spanIDs := make([]SpanID, n)
	for i := 0; i < n; i++ {
		var parent ID
		if i != 0 {
			parent = ID(rand.Intn(n) + 1)
		}
		spanIDs[i] = SpanID{1, ID(i + 1), parent}
	}

	t.Logf("collecting %d spans, checking for errors and panics", n)
	for _, spanID := range spanIDs {
		ms.MustCollect(spanID)
	}

	x := ms.MustTrace(1)
	if want := (SpanID{1, 1, 0}); x.Span.ID != want {
		t.Errorf("Trace(1): got SpanID %+v, want %+v", x.Span.ID, want)
	}
}

var (
	traceTreePerm = flag.Int("test.trace-tree-perm", -1, "if > 0, only run this permutation in TestMemoryStore_Collect_traceTreeRearrangement")
)

func TestMemoryStore_Collect_traceTreeRearrangement(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	const (
		spans = 50
		perms = 10
	)

	for n := 1; n <= spans; n++ {
		t.Logf("testing rearrangement with n=%d spans, %d permutations", n, perms)
		for seed := int64(0); seed < perms; seed++ {
			if *traceTreePerm != -1 && int64(*traceTreePerm) != seed {
				continue
			}

			ms := storeT{t, NewMemoryStore()}
			// ms.Store.(*MemoryStore).log = true

			perm := rand.New(rand.NewSource(seed)).Perm(n)
			spanIDs := make([]SpanID, n) // random order
			traces := make(map[ID]*Trace, n)
			for j := 0; j < n; j++ {
				var parent ID
				if j == 0 {
					parent = 0 // root
				} else if j == 1 {
					parent = ID(1) // parent is root
				} else if j <= n/2 {
					parent = ID(j - 1) // parent is predecessor
				} else {
					parent = ID(n / 2) // fixed parent
				}
				id := SpanID{1, ID(j + 1), parent}
				spanIDs[perm[j]] = id
				traces[id.Span] = &Trace{Span: Span{ID: id}}
			}
			for _, x := range traces {
				parentID := x.Span.ID.Parent
				if parentID == 0 {
					continue // root
				}
				parent, present := traces[parentID]
				if !present {
					t.Fatalf("parent %v not found", uint64(parentID))
				}
				parent.Sub = append(parent.Sub, x)
			}

			// t.Logf("collecting %d spans in random order", n)
			for _, spanID := range spanIDs {
				ms.MustCollect(spanID)
			}

			want := traces[1] // root
			x := ms.MustTrace(1)

			// Sort order of children for comparison.
			want.sortSubRecursive()
			x.sortSubRecursive()

			if diff := compareTraces(x, want); len(diff) > 0 {
				t.Errorf("seed=%d: Trace(1): trees differed:\n%s\n\ngot tree\n%s\n\nwant tree\n%s", seed, strings.Join(diff, "\n"), x, want)
			}
		}
	}
}

func (t *Trace) sortSubRecursive() {
	sort.Sort(tracesByIDSpan(t.Sub))
	for _, st := range t.Sub {
		st.sortSubRecursive()
	}
}

func TestRecentStore(t *testing.T) {
	const age = time.Millisecond * 10

	ms := NewMemoryStore()
	rs := &storeT{t, &RecentStore{DeleteStore: ms, MinEvictAge: age}}

	rs.MustCollect(SpanID{1, 2, 3})
	rs.MustCollect(SpanID{2, 3, 4})

	traces, _ := ms.Traces(TracesOpts{})
	if len(traces) != 2 {
		t.Errorf("got traces %v, want %d total", traces, 2)
	}

	time.Sleep(2 * age)
	rs.MustCollect(SpanID{3, 4, 5})
	time.Sleep(2 * age)
	traces, _ = ms.Traces(TracesOpts{})
	if len(traces) != 1 {
		t.Errorf("got traces %v, want %d total", traces, 1)
	}
	if trace, want := traces[0].ID, (SpanID{3, 4, 5}); trace != want {
		t.Errorf("got trace %v, want %v", trace, want)
	}
}

func TestLimitStore(t *testing.T) {
	const age = time.Millisecond * 10

	ms := NewMemoryStore()
	rs := &storeT{t, &LimitStore{DeleteStore: ms, Max: 2}}

	if traces, _ := ms.Traces(TracesOpts{}); len(traces) != 0 {
		t.Errorf("got traces %v, want %d total", traces, 0)
	}

	rs.MustCollect(SpanID{1, 2, 3})

	if traces, _ := ms.Traces(TracesOpts{}); len(traces) != 1 {
		t.Errorf("got traces %v, want %d total", traces, 1)
	}

	rs.MustCollect(SpanID{2, 3, 4})

	if traces, _ := ms.Traces(TracesOpts{}); len(traces) != 2 {
		t.Errorf("got traces %v, want %d total", traces, 2)
	}

	rs.MustCollect(SpanID{3, 4, 5})
	rs.MustCollect(SpanID{3, 5, 6})

	if traces, _ := ms.Traces(TracesOpts{}); len(traces) != 2 {
		t.Errorf("got traces %v, want %d total", traces, 2)
	}

	traces, _ := ms.Traces(TracesOpts{})
	want := []*Trace{
		{Span: Span{ID: SpanID{2, 3, 4}}},
		{
			Span: Span{ID: SpanID{3, 5, 6}},
			Sub: []*Trace{
				{Span: Span{ID: SpanID{3, 4, 5}}},
			},
		},
	}
	sort.Sort(tracesByIDSpan(traces))
	if !reflect.DeepEqual(traces, want) {
		t.Errorf("traces differed\n\ngot traces\n%s\n\nwant traces\n%s", traces, want)
	}
}

func compareTraces(a, b *Trace) (diff []string) {
	var cmp func(parent ID, a, b *Trace)
	cmp = func(parent ID, a, b *Trace) {
		same := true
		if !reflect.DeepEqual(a.Span, b.Span) {
			diff = append(diff, fmt.Sprintf("%x: spans differed: %+v != %+v", uint64(parent), a.Span, b.Span))
			same = false
		}
		if len(a.Sub) != len(b.Sub) {
			diff = append(diff, fmt.Sprintf("%x: children len differed: %d != %d", uint64(parent), len(a.Sub), len(b.Sub)))
			same = false
		}
		if same {
			for i, asub := range a.Sub {
				bsub := b.Sub[i]
				cmp(a.Span.ID.Span, asub, bsub)
			}
		}
	}
	cmp(0, a, b)
	return diff
}

type storeT struct {
	t *testing.T
	Store
}

func (s storeT) MustCollect(id SpanID, as ...Annotation) {
	if err := s.Store.Collect(id, as...); err != nil {
		s.t.Fatalf("Collect(%+v, %v): %s", id, as, err)
	}
}

func (s storeT) MustTrace(id ID) *Trace {
	t, err := s.Store.Trace(id)
	if err != nil {
		s.t.Fatalf("Trace(%v): %s", id, err)
	}
	return t
}

func init() {
	log.SetFlags(0)
}

func benchmarkMemoryStoreN(b *testing.B, n int) {
	ms := NewMemoryStore()
	var x ID
	for i := 0; i < b.N; i++ {
		for c := 0; c < n; c++ {
			x++
			err := ms.Collect(SpanID{x, x + 1, x + 2})
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkMemoryStore100(b *testing.B) {
	benchmarkMemoryStoreN(b, 100)
}

func BenchmarkMemoryStore250(b *testing.B) {
	benchmarkMemoryStoreN(b, 250)
}

func BenchmarkMemoryStore1000(b *testing.B) {
	benchmarkMemoryStoreN(b, 1000)
}

func BenchmarkMemoryStoreWrite1000(b *testing.B) {
	ms := NewMemoryStore()
	var x ID
	for c := 0; c < 1000; c++ {
		x++
		err := ms.Collect(SpanID{x, x + 1, x + 2})
		if err != nil {
			b.Fatal(err)
		}
	}

	for i := 0; i < b.N; i++ {
		err := ms.Write(ioutil.Discard)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryStoreReadFrom1000(b *testing.B) {
	ms := NewMemoryStore()
	var x ID
	for c := 0; c < 1000; c++ {
		x++
		err := ms.Collect(SpanID{x, x + 1, x + 2})
		if err != nil {
			b.Fatal(err)
		}
	}
	buf := bytes.NewBuffer(nil)
	err := ms.Write(buf)
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		tmp := NewMemoryStore()
		_, err := tmp.ReadFrom(bytes.NewReader(buf.Bytes()))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRecentStore500(b *testing.B) {
	const (
		nCollections = 500
		nAnnotations = 100
	)
	rs := &RecentStore{
		DeleteStore: NewMemoryStore(),
		MinEvictAge: 20 * time.Second,
	}
	var x ID
	for i := 0; i < b.N; i++ {
		for c := 0; c < nCollections; c++ {
			x++
			anns := make([]Annotation, nAnnotations)
			for a := range anns {
				anns[a] = Annotation{"k1", []byte("v1")}
			}
			err := rs.Collect(SpanID{x, 2, 3}, anns...)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkLimitStore500(b *testing.B) {
	const (
		nCollections = 500
		nAnnotations = 100
	)
	rs := &LimitStore{
		DeleteStore: NewMemoryStore(),
		Max:         2000,
	}
	var x ID
	for i := 0; i < b.N; i++ {
		for c := 0; c < nCollections; c++ {
			x++
			anns := make([]Annotation, nAnnotations)
			for a := range anns {
				anns[a] = Annotation{"k1", []byte("v1")}
			}
			err := rs.Collect(SpanID{x, 2, 3}, anns...)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
