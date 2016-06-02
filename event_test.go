package appdash

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
		E: "e",
		F: dummyEventF{
			G: "g",
			H: map[string]string{"k3": "v3", "k4": "v4"},
		},
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
		{Key: "e", Value: []byte("e")},
		{Key: "F.G", Value: []byte("g")},
		{Key: "F.H.k3", Value: []byte("v3")},
		{Key: "F.H.k4", Value: []byte("v4")},
		{Key: "_schema:dummy"},
	}

	sort.Sort(annotations(as))
	sort.Sort(annotations(want))

	if !reflect.DeepEqual(as, want) {
		t.Errorf("got annotations\n%s\n\nwant\n%s", as, want)
	}
}

func TestUnmarshalEvent(t *testing.T) {
	as := Annotations{
		{Key: "A", Value: []byte("a")},
		{Key: "B", Value: []byte("b")},
		{Key: "C", Value: []byte("1")},
		{Key: "D.k1", Value: []byte("v1")},
		{Key: "D.k2", Value: []byte("v2")},
		{Key: "e", Value: []byte("e")},
		{Key: "F.G", Value: []byte("g")},
		{Key: "F.H.k3", Value: []byte("v3")},
		{Key: "F.H.k4", Value: []byte("v4")},
		{Key: "_schema:dummy"},
	}

	var e dummyEvent
	if err := UnmarshalEvent(as, &e); err != nil {
		t.Fatal(err)
	}

	want := dummyEvent{
		A: "a",
		B: "b",
		C: 1,
		D: map[string]string{"k1": "v1", "k2": "v2"},
		E: "e",
		F: dummyEventF{
			G: "g",
			H: map[string]string{"k3": "v3", "k4": "v4"},
		},
	}

	if !reflect.DeepEqual(e, want) {
		t.Errorf("got event\n%+v\n\nwant\n%+v", e, want)
	}
}

func TestUnmarshalEvents(t *testing.T) {
	origRegisteredEvents := registeredEvents
	defer func() {
		registeredEvents = origRegisteredEvents
	}()
	registeredEvents = make(map[string]Event)

	RegisterEvent(dummyEvent{})
	RegisterEvent(dummyEvent2{})

	anns := Annotations{
		{Key: "A", Value: []byte("a")},
		{Key: "X", Value: []byte("x")},
		{Key: "_schema:dummy"},
		{Key: "_schema:dummy2"},
	}
	var events []Event
	if err := UnmarshalEvents(anns, &events); err != nil {
		t.Fatal(err)
	}

	want := []Event{
		dummyEvent{A: "a"},
		dummyEvent2{A: "a", X: "x"},
	}
	if !reflect.DeepEqual(events, want) {
		t.Errorf("got events %#v, want %#v", events, want)
	}
}

func TestSpanName(t *testing.T) {
	e := SpanNameEvent{"foo"}

	anns, err := MarshalEvent(e)
	if err != nil {
		t.Fatal(err)
	}

	span := Span{Annotations: anns}
	if want := "foo"; span.Name() != want {
		t.Errorf("got span name == %q, want %q", span.Name(), want)
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
	E    string `trace:"e"`
	F    dummyEventF
}

type dummyEventF struct {
	G string
	H map[string]string
}

func (dummyEvent) Schema() string { return "dummy" }

type dummyEvent2 struct{ A, X string }

func (dummyEvent2) Schema() string { return "dummy2" }

func BenchmarkMarshalEvent(b *testing.B) {
	e := dummyEvent{
		A: "a",
		B: "b",
		C: 1,
		D: map[string]string{"k1": "v1", "k2": "v2"},
		E: "e",
		F: dummyEventF{
			G: "g",
			H: map[string]string{"k3": "v3", "k4": "v4"},
		},
	}
	for i := 0; i < b.N; i++ {
		_, err := MarshalEvent(e)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalEvents(b *testing.B) {
	anns := Annotations{
		{Key: "A", Value: []byte("a")},
		{Key: "X", Value: []byte("x")},
		{Key: "_schema:dummy"},
		{Key: "_schema:dummy2"},
	}
	for i := 0; i < b.N; i++ {
		var events []Event
		if err := UnmarshalEvents(anns, &events); err != nil {
			b.Fatal(err)
		}
	}
}
