package appdash

import (
	"testing"
	"time"
)

// Verify the event type satisfies the interfaces.
var _ = EventMarshaler(AggregateEvent{})
var _ = EventUnmarshaler(AggregateEvent{})
var _ = Event(AggregateEvent{})

// fakeTimespan represents a fake timespan event, and is used for the tests
// below.
type fakeTimespan struct {
	S, E time.Time
}

func (f fakeTimespan) Schema() string   { return "fake" }
func (f fakeTimespan) Start() time.Time { return f.S }
func (f fakeTimespan) End() time.Time   { return f.E }

var _ = TimespanEvent(fakeTimespan{})

func init() { RegisterEvent(fakeTimespan{}) }

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
		e := fakeTimespan{
			S: time.Now().Add(time.Duration(-i) * time.Minute),
			E: time.Now(),
		}
		rec.Event(e)
		if errs := rec.Errors(); len(errs) > 0 {
			t.Fatal(errs)
		}
	}

	// Verify the recorded traces.
	traces, err := ms.Traces()
	if err != nil {
		t.Fatal(err)
	}

	// One trace is the aggregated one, the other 5 are the N-slowest full
	// traces.
	if len(traces) != 6 {
		t.Fatalf("expected 6 traces got %d", len(traces))
	}

	// Verify we have the aggregated trace events.
	var agg []AggregateEvent
	for _, tr := range traces {
		evs, err := tr.Aggregated()
		if err != nil {
			t.Fatal(err)
		}
		if len(evs) > 0 {
			agg = evs
		}
	}
	if len(agg) != 1 {
		t.Fatalf("expected 1 aggregated trace event, found %d", len(agg))
	}

	// Verify we have the N-slowest other full traces.
	var found []ID
	for _, t := range traces {
		for _, want := range agg[0].Slowest {
			if t.Span.ID.Trace == want {
				found = append(found, want)
			}
		}
	}
	if len(found) != as.NSlowest {
		t.Fatalf("expected %d N-slowest full traces, found %d", as.NSlowest, len(found))
	}
}
