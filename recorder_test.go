package appdash

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestRecorder(t *testing.T) {
	id := SpanID{1, 2, 3}

	calledCollect := 0
	var anns Annotations
	c := collectorFunc(func(spanID SpanID, as ...Annotation) error {
		calledCollect++
		if spanID != id {
			t.Errorf("Collect: got spanID arg %v, want %v", spanID, id)
		}
		anns = append(anns, as...)
		return nil
	})

	r := NewRecorder(id, c)

	r.Msg("msg")
	r.Name("name")

	if calledCollect != 0 {
		t.Errorf("got calledCollect %d, want 0", calledCollect)
	}

	r.Finish()

	if diff := diffAnnotationsFromEvent(anns, Msg("msg")); len(diff) > 0 {
		t.Errorf("got diff annotations for Msg event:\n%s", strings.Join(diff, "\n"))
	}

	if diff := diffAnnotationsFromEvent(anns, SpanNameEvent{"name"}); len(diff) > 0 {
		t.Errorf("got diff annotations for SpanNameEvent:\n%s", strings.Join(diff, "\n"))
	}
}

func TestRecorder_Errors(t *testing.T) {
	collectErr := errors.New("Collect error")
	calledCollect := 0
	c := collectorFunc(func(spanID SpanID, as ...Annotation) error {
		calledCollect++
		return collectErr
	})

	r := NewRecorder(SpanID{}, c)

	r.Msg("msg")
	if calledCollect != 0 {
		t.Errorf("got calledCollect %d, want 0", calledCollect)
	}
	r.Finish()
	errs := r.Errors()
	if want := []error{collectErr}; !reflect.DeepEqual(errs, want) {
		t.Errorf("got errors %v, want %v", errs, want)
	}

	if errs := r.Errors(); len(errs) != 0 {
		t.Errorf("got len(errs) == %d, want 0 (after call to Errors)", len(errs))
	}

	r.Finish()
	if errs := r.Errors(); len(errs) != 1 && errs[0] != errMultipleFinishCalls {
		t.Errorf("got %d errors, want 1", len(errs))
	}
}

func diffAnnotationsFromEvent(anns Annotations, e Event) (diff []string) {
	eventAnns, err := MarshalEvent(e)
	if err != nil {
		panic(err)
	}

	matchesEventAnns := map[string]bool{}
	for _, ea := range eventAnns {
		for _, a := range anns {
			if ea.Key == a.Key && bytes.Equal(ea.Value, a.Value) {
				matchesEventAnns[ea.Key] = true
			}
		}
	}

	for _, ea := range eventAnns {
		if !matchesEventAnns[ea.Key] {
			diff = append(diff, fmt.Sprintf("key %s: %q != %q", ea.Key, ea.Value, anns.get(ea.Key)))
		}
	}
	return diff
}
