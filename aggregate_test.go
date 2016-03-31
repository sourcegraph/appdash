package appdash

import (
	"reflect"
	"testing"
	"time"
)

// Verify the event type satisfies the interfaces.
var _ = EventMarshaler(AggregateEvent{})
var _ = EventUnmarshaler(AggregateEvent{})
var _ = Event(AggregateEvent{})

// TestAggregateStore tests basic AggregateStore functionality.
func TestAggregateStore(t *testing.T) {
	// Create an aggregate store.
	ms := NewMemoryStore()
	as := &AggregateStore{
		MinEvictAge: 72 * time.Hour,
		MaxRate:     4096,
		NSlowest:    5,
		MemoryStore: ms,
	}

	// Record a few traces under the same name.
	for i := 0; i < 10; i++ {
		root := NewRootSpanID()
		rec := NewRecorder(root, as)
		rec.Name("the-trace-name")
		e := timespanEvent{
			S: time.Now().Add(time.Duration(-i) * time.Minute),
			E: time.Now(),
		}
		rec.Event(e)
		rec.Finish()
		if errs := rec.Errors(); len(errs) > 0 {
			t.Fatal(errs)
		}
	}

	// Verify the recorded traces.
	traces, err := ms.Traces(TracesOpts{})
	if err != nil {
		t.Fatal(err)
	}

	// One trace is the aggregated one, the other 5 are the N-slowest full
	// traces.
	if len(traces) != 6 {
		t.Fatalf("expected 6 traces got %d", len(traces))
	}

	// Verify we have the aggregated trace events.
	var agg *AggregateEvent
	for _, tr := range traces {
		ev, _, err := tr.Aggregated()
		if err != nil {
			t.Fatal(err)
		}
		if ev != nil {
			agg = ev
		}
	}
	if agg == nil {
		t.Fatalf("expected 1 aggregated trace event, found nil")
	}

	// Verify we have the N-slowest other full traces.
	var found []ID
	for _, t := range traces {
		for _, want := range agg.Slowest {
			if t.Span.ID.Trace == want {
				found = append(found, want)
			}
		}
	}
	if len(found) != as.NSlowest {
		t.Fatalf("expected %d N-slowest full traces, found %d", as.NSlowest, len(found))
	}
}

// TestAggregateStoreNSlowest tests that the AggregateStore.NSlowest field is
// operating correctly.
func TestAggregateStoreNSlowest(t *testing.T) {
	// Create an aggregate store.
	ms := NewMemoryStore()
	as := &AggregateStore{
		MinEvictAge: 72 * time.Hour,
		MaxRate:     4096,
		NSlowest:    5,
		MemoryStore: ms,
	}

	now := time.Now()

	insert := func(times []time.Duration) []time.Duration {
		// Record a few traces under the same name.
		for i := 0; i < len(times); i++ {
			root := NewRootSpanID()
			rec := NewRecorder(root, as)
			rec.Name("the-trace-name")
			e := timespanEvent{
				S: now,
				E: now.Add(times[i]),
			}
			rec.Event(e)
			rec.Finish()
			if errs := rec.Errors(); len(errs) > 0 {
				t.Fatal(errs)
			}
		}

		// Query the traces from the memory store.
		traces, err := ms.Traces(TracesOpts{})
		if err != nil {
			t.Fatal(err)
		}

		// One trace is the aggregated one, the other 5 are the N-slowest full
		// traces.
		if len(traces) != as.NSlowest+1 {
			t.Fatalf("expected %d traces got %d", as.NSlowest+1, len(traces))
		}

		// Verify we have the aggregated trace events.
		var agg *AggregateEvent
		for _, tr := range traces {
			ev, _, err := tr.Aggregated()
			if err != nil {
				t.Fatal(err)
			}
			if ev != nil {
				agg = ev
			}
		}
		if agg == nil {
			t.Fatalf("expected 1 aggregated trace event, found nil")
		}

		// Determine time of each slowest trace.
		var d []time.Duration
		for _, slowest := range agg.Slowest {
			st, err := ms.Trace(slowest)
			if err != nil {
				t.Fatal(err)
			}

			// Unmarshal the events.
			var events []Event
			if err := UnmarshalEvents(st.Annotations, &events); err != nil {
				t.Fatal(err)
			}

			start, end, ok := findTraceTimes(events)
			if !ok {
				t.Fatal("no timespane events")
			}
			d = append(d, end.Sub(start))
		}
		return d
	}

	// Insert ten basic values to start with.
	want := []time.Duration{
		5 * time.Minute,
		5 * time.Minute,
		4 * time.Minute,
		4 * time.Minute,
		3 * time.Minute,
	}
	got := insert([]time.Duration{
		2 * time.Minute,
		3 * time.Minute,
		5 * time.Minute,
		4 * time.Minute,
		1 * time.Minute,
		4 * time.Minute,
		2 * time.Minute,
		5 * time.Minute,
		3 * time.Minute,
		1 * time.Minute,
	})
	if !reflect.DeepEqual(got, want) {
		t.Logf("got %v\n", got)
		t.Fatalf("want %v", want)
	}

	// Now we insert a sixth value which should overtake the smallest duration.
	want = []time.Duration{
		6 * time.Minute,
		5 * time.Minute,
		5 * time.Minute,
		4 * time.Minute,
		4 * time.Minute,
	}
	got = insert([]time.Duration{6 * time.Minute})
	if !reflect.DeepEqual(got, want) {
		t.Logf("got %v\n", got)
		t.Fatalf("want %v", want)
	}
}

func TestAggregateStoreMinEvictAge(t *testing.T) {
	// Create an aggregate store.
	ms := NewMemoryStore()
	as := &AggregateStore{
		MinEvictAge: 1 * time.Second,
		MaxRate:     4096,
		NSlowest:    5,
		MemoryStore: ms,
	}

	// Record a few traces.
	for i := 0; i < 10; i++ {
		root := NewRootSpanID()
		rec := NewRecorder(root, as)
		rec.Name("the-trace-name")
		e := timespanEvent{
			S: time.Now().Add(time.Duration(-i) * time.Minute),
			E: time.Now(),
		}
		rec.Event(e)
		rec.Finish()
		if errs := rec.Errors(); len(errs) > 0 {
			t.Fatal(errs)
		}
	}

	// Verify the number of recorded traces.
	traces, err := ms.Traces(TracesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(traces) != 6 {
		t.Fatalf("expected 6 traces got %d", len(traces))
	}

	// Wait so that next collection will cause eviction.
	time.Sleep(as.MinEvictAge)

	// Trigger the eviction by making any sort of collection.
	rec := NewRecorder(NewRootSpanID(), as)
	rec.Name("collect")
	rec.Finish()
	if errs := rec.Errors(); len(errs) > 0 {
		t.Fatal(errs)
	}

	// Wait for deletion to occur (it happens in a separate goroutine and we
	// don't want to introduce synchronization just for this test).
	time.Sleep(1 * time.Second)

	// Verify the eviction.
	traces, err = ms.Traces(TracesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(traces) != 0 {
		t.Fatalf("expected 0 traces got %d", len(traces))
	}
}
