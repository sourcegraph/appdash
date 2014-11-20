package apptrace

import (
	"reflect"
	"testing"
	"time"
)

func TestFlattenBools(t *testing.T) {
	e := struct {
		Value bool
	}{
		Value: true,
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"value": "true",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFlattenStrings(t *testing.T) {
	e := struct {
		Value string
	}{
		Value: "bar",
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"value": "bar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFlattenNamedValues(t *testing.T) {
	e := struct {
		Value string `trace:"foo"`
	}{
		Value: "bar",
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"foo": "bar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFlattenTime(t *testing.T) {
	e := struct {
		Value time.Time
	}{
		Value: time.Date(2014, 5, 16, 12, 28, 38, 400, time.UTC),
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"value": "2014-05-16T12:28:38.0000004Z",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFlattenFloats(t *testing.T) {
	e := struct {
		A float32
		B float64
	}{
		A: 3,
		B: 500.3,
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"a": "3",
		"b": "500.3",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFlattenInts(t *testing.T) {
	e := struct {
		A int8
		B int16
		C int32
		D int64
		E int
	}{
		A: 1,
		B: 2,
		C: 3,
		D: 4,
		E: 5,
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
		"d": "4",
		"e": "5",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFlattenUints(t *testing.T) {
	e := struct {
		A uint8
		B uint16
		C uint32
		D uint64
		E uint
	}{
		A: 1,
		B: 2,
		C: 3,
		D: 4,
		E: 5,
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
		"d": "4",
		"e": "5",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFlattenMaps(t *testing.T) {
	e := struct {
		Value map[string]int
	}{
		Value: map[string]int{
			"one": 1,
			"two": 2,
		},
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"value.one": "1",
		"value.two": "2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFlattenSlices(t *testing.T) {
	e := struct {
		Value []int
	}{
		Value: []int{1, 2, 3},
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"value.0": "1",
		"value.1": "2",
		"value.2": "3",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFlattenArrays(t *testing.T) {
	e := struct {
		Value [3]int
	}{
		Value: [3]int{1, 2, 3},
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"value.0": "1",
		"value.1": "2",
		"value.2": "3",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

type stringer byte

func (stringer) String() string {
	return "stringer"
}

func TestFlattenStringers(t *testing.T) {
	e := struct {
		Value stringer
	}{
		Value: 30,
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"value": "stringer",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFlattenArbitraryTypes(t *testing.T) {
	e := struct {
		Value complex64
	}{
		Value: complex(17, 4),
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"value": "(17+4i)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFlattenUnexportedFields(t *testing.T) {
	e := struct {
		value string
	}{
		value: "bar",
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFlattenCacheFields(t *testing.T) {
	e := struct{}{}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	got = make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want = map[string]string{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFlattenDuration(t *testing.T) {
	e := struct {
		Value time.Duration
	}{
		Value: 500 * time.Microsecond,
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"value": "0.5",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFlattenPointers(t *testing.T) {
	s := "bar"
	e := struct {
		Value *string
	}{
		Value: &s,
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"value": "bar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

type testInnerEvent struct {
	Days  map[string]int
	Other []bool
}

type testEvent struct {
	Name   string `lunk:"nombre"`
	Age    int
	Inner  testInnerEvent
	Weight float64
	Count  uint
	turds  *byte
}

func BenchmarkFlatten(b *testing.B) {
	e := testEvent{
		Name: "hello",
		Age:  400,
		Inner: testInnerEvent{
			Days: map[string]int{
				"Sunday": 1,
			},
			Other: []bool{true, false},
		},
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		flattenValue("", reflect.ValueOf(e), func(k, v string) {})
	}
}
