package appdash

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
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

// SlowestRawQuery creates a list of slowest trace IDs (but as strings),
// then produce a URL which will query for it.
func (e AggregateEvent) SlowestRawQuery() string {
	var stringIDs []string
	for _, slowest := range e.Slowest {
		stringIDs = append(stringIDs, slowest.String())
	}
	return "show=" + strings.Join(stringIDs, ",")
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
	Trace     SpanID
	Name      string             // Root span name (e.g. the route for httptrace).
	Times     [][2]time.Time     // Aggregated timespans for the traces.
	TimeSpans []ID               // SpanID.Span of each associated TimespanEvent for the Times slice
	Slowest   []spanGroupSlowest // N-slowest traces in the group.
}

func (s spanGroup) Len() int      { return len(s.Slowest) }
func (s spanGroup) Swap(i, j int) { s.Slowest[i], s.Slowest[j] = s.Slowest[j], s.Slowest[i] }
func (s spanGroup) Less(i, j int) bool {
	a := s.Slowest[i]
	b := s.Slowest[j]

	// A sorts before B if it took a greater amount of time than B (slowest
	// to-fastest sorting).
	return a.End.Sub(a.Start) > b.End.Sub(b.Start)
}

// update updates the span group to account for a potentially slowest trace,
// returning whether or not the given trace was indeed slowest. The timespan ID
// is the SpanID.Span of the TimespanEvent for future removal upon eviction.
func (s *spanGroup) update(start, end time.Time, timespan ID, trace ID, remove func(trace ID)) bool {
	s.Times = append(s.Times, [2]time.Time{start, end})
	s.TimeSpans = append(s.TimeSpans, timespan)

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
func (s *spanGroup) evictBefore(tnano int64, debug bool, deleteSub func(s SpanID)) {
	count := 0
search:
	for i, ts := range s.Times {
		if ts[0].UnixNano() < tnano {
			s.Times = append(s.Times[:i], s.Times[i+1:]...)

			// Remove the associated subspan holding the TimespanEvent in the
			// output MemoryStore.
			id := s.TimeSpans[i]
			s.TimeSpans = append(s.TimeSpans[:i], s.TimeSpans[i+1:]...)
			deleteSub(SpanID{Trace: s.Trace.Trace, Span: id, Parent: s.Trace.Span})

			count++
			goto search
		}
	}

	if debug && count > 0 {
		log.Printf("AggregateStore: evicted %d timespans from the group %q", count, s.Name)
	}
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
