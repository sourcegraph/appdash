package opentracing

import (
	"reflect"
	"sort"
	"testing"
	"time"

	basictracer "github.com/opentracing/basictracer-go"

	"sourcegraph.com/sourcegraph/appdash"
	"sourcegraph.com/sourcegraph/appdash/internal/wire"
)

func TestOpentracingRecorder(t *testing.T) {
	var packets []*wire.CollectPacket
	mc := collectorFunc(func(span appdash.SpanID, anns ...appdash.Annotation) error {
		packets = append(packets, newCollectPacket(span, anns))
		return nil
	})

	baggageKey := "somelongval"
	baggageVal := "val"
	opName := "testOperation"

	r := NewRecorder(mc, Options{})
	raw := basictracer.RawSpan{
		Context: basictracer.SpanContext{
			TraceID: 1,
			SpanID:  2,
			Sampled: true,
			Baggage: map[string]string{
				baggageKey: baggageVal,
			},
		},
		ParentSpanID: 3,
		Tags: map[string]interface{}{
			"tag": 1,
		},
		Operation: opName,
		Duration:  time.Duration(1),
	}

	unsampledRaw := basictracer.RawSpan{
		Context: basictracer.SpanContext{
			TraceID: 1,
			SpanID:  2,
			Sampled: false,
		},
		ParentSpanID: 3,
	}

	r.RecordSpan(raw)
	r.RecordSpan(unsampledRaw)

	tsAnnotations := marshalEvent(appdash.Timespan{raw.Start, raw.Start.Add(raw.Duration)})
	want := []*wire.CollectPacket{
		newCollectPacket(appdash.SpanID{1, 2, 3}, appdash.Annotations{{"tag", []byte("1")}}),
		newCollectPacket(appdash.SpanID{1, 2, 3}, appdash.Annotations{{baggageKey, []byte(baggageVal)}}),
		newCollectPacket(appdash.SpanID{1, 2, 3}, marshalEvent(appdash.SpanName(opName))),
		newCollectPacket(appdash.SpanID{1, 2, 3}, tsAnnotations),
	}

	sort.Sort(byTraceID(packets))
	sort.Sort(byTraceID(want))
	if !reflect.DeepEqual(packets, want) {
		t.Errorf("Got packets %v, want %v", packets, want)
	}
}

// newCollectPacket returns an initialized *wire.CollectPacket given a span and
// set of annotations.
func newCollectPacket(s appdash.SpanID, as appdash.Annotations) *wire.CollectPacket {
	swire := &wire.CollectPacket_SpanID{
		Trace:  (*uint64)(&s.Trace),
		Span:   (*uint64)(&s.Span),
		Parent: (*uint64)(&s.Parent),
	}
	w := []*wire.CollectPacket_Annotation{}

	for _, a := range as {
		// Important: Make a copy of a that we can retain a pointer to that
		// doesn't change after each iteration. Otherwise all wire annotations
		// would have the same key.
		cpy := a
		w = append(w, &wire.CollectPacket_Annotation{
			Key:   &cpy.Key,
			Value: cpy.Value,
		})
	}
	return &wire.CollectPacket{
		Spanid:     swire,
		Annotation: w,
	}
}

type collectorFunc func(appdash.SpanID, ...appdash.Annotation) error

// Collect implements the Collector interface by calling the function itself.
func (c collectorFunc) Collect(id appdash.SpanID, as ...appdash.Annotation) error {
	return c(id, as...)
}

func marshalEvent(e appdash.Event) appdash.Annotations {
	ans, _ := appdash.MarshalEvent(e)
	return ans
}

type byTraceID []*wire.CollectPacket

func (bt byTraceID) Len() int      { return len(bt) }
func (bt byTraceID) Swap(i, j int) { bt[i], bt[j] = bt[j], bt[i] }
func (bt byTraceID) Less(i, j int) bool {
	if *bt[i].Spanid.Trace < *bt[j].Spanid.Trace {
		return true
	}

	// If the packet has more than one annotation, sort those annotation by name.
	if len(bt[i].Annotation) > 1 {
		sort.Sort(byAnnotation(bt[i].Annotation))
	}
	if len(bt[j].Annotation) > 1 {
		sort.Sort(byAnnotation(bt[i].Annotation))
	}
	return *bt[i].Annotation[0].Key < *bt[j].Annotation[0].Key
}

type byAnnotation []*wire.CollectPacket_Annotation

func (bt byAnnotation) Len() int           { return len(bt) }
func (bt byAnnotation) Swap(i, j int)      { bt[i], bt[j] = bt[j], bt[i] }
func (bt byAnnotation) Less(i, j int) bool { return *bt[i].Key < *bt[j].Key }
