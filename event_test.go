package apptrace

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestMarshalEvent(t *testing.T) {
	e := dummyEvent{
		A: "a",
		B: "b",
		C: 1,
		D: map[string]string{"k1": "v1", "k2": "v2"},
	}
	as, err := MarshalEvent(e)
	if err != nil {
		t.Fatal(err)
	}

	want := Annotations{
		{Key: "A", Value: []byte("a")},
		{Key: "B", Value: []byte("b")},
		{Key: "C", Value: []byte("1")},
		{Key: "D.k1", Value: []byte("v1")},
		{Key: "D.k2", Value: []byte("v2")},
		{Key: "_schema:dummy"},
	}

	sort.Sort(annotations(as))
	sort.Sort(annotations(want))

	if !reflect.DeepEqual(as, want) {
		t.Errorf("got annotations\n%s\n\nwant\n%s", as, want)
	}
}

func TestUnmarshalEvent(t *testing.T) {
	// TODO(sqs): add C/D.k1/D.k2 props to test case (and other
	// non-string types) when support is implemented.

	as := Annotations{
		{Key: "A", Value: []byte("a")},
		{Key: "B", Value: []byte("b")},
		{Key: "C", Value: []byte("1")},
		{Key: "_schema:dummy"},
	}

	var e dummyEvent
	if err := UnmarshalEvent(as, &e); err != nil {
		t.Fatal(err)
	}

	want := dummyEvent{
		A: "a",
		B: "b",
	}

	if !reflect.DeepEqual(e, want) {
		t.Errorf("got event\n%+v\n\nwant\n%+v", e, want)
	}
}

func TestMsg(t *testing.T) {
	e := Msg("foo")

	j, err := json.Marshal(e)
	if err != nil {
		t.Error(err)
	}

	got := string(j)
	want := `{"Msg":"foo"}`
	if got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestMsg_Schema(t *testing.T) {
	e := Msg("foo")

	if e.Schema() != "msg" {
		t.Errorf("Unwant schema: %v", e.Schema())
	}
}

func TestLog(t *testing.T) {
	e := Log("foo").(logEvent)

	e.Time = time.Unix(123456789, 0).In(time.UTC)

	j, err := json.Marshal(e)
	if err != nil {
		t.Error(err)
	}

	got := string(j)
	want := `{"Msg":"foo","Time":"1973-11-29T21:33:09Z"}`
	if got != want {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestLog_Schema(t *testing.T) {
	e := Log("foo")

	if e.Schema() != "log" {
		t.Errorf("Unwant schema: %v", e.Schema())
	}
}

type dummyEvent struct {
	A, B string
	C    int
	D    map[string]string
}

func (dummyEvent) Schema() string { return "dummy" }
