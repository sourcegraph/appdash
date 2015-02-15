package appdash

import (
	"math"
	"reflect"
	"testing"
	"time"
)

func TestFlattenBools(t *testing.T) {
	type T struct {
		Value bool
	}
	e := T{
		Value: true,
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"Value": "true",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(want)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotE, e) {
		t.Errorf("got %#v, want %#v", gotE, e)
	}
}

func TestFlattenStrings(t *testing.T) {
	type T struct {
		Value string
	}
	e := T{
		Value: "bar",
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"Value": "bar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(want)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotE, e) {
		t.Errorf("got %#v, want %#v", gotE, e)
	}
}

func TestFlattenNamedValues(t *testing.T) {
	type T struct {
		Value string `trace:"foo"`
	}
	e := T{
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

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(want)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotE, e) {
		t.Errorf("got %#v, want %#v", gotE, e)
	}
}

func TestFlattenTime(t *testing.T) {
	type T struct {
		Value time.Time
	}
	e := T{
		Value: time.Date(2014, 5, 16, 12, 28, 38, 400, time.UTC),
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"Value": "2014-05-16T12:28:38.0000004Z",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(want)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotE, e) {
		t.Errorf("got %#v, want %#v", gotE, e)
	}
}

func TestFlattenFloats(t *testing.T) {
	type T struct {
		A float32
		B float64
	}
	e := T{
		A: 3,
		B: 500.3,
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"A": "3",
		"B": "500.3",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(want)); err != nil {
		t.Fatal(err)
	}
	if math.Abs(gotE.B-e.B) > 0.01 {
		t.Errorf("got B = %f, want %f", gotE.B, e.B)
	}
	gotE.B = e.B
	if !reflect.DeepEqual(gotE, e) {
		t.Errorf("got %#v, want %#v", gotE, e)
	}
}

func TestFlattenInts(t *testing.T) {
	type T struct {
		A int8
		B int16
		C int32
		D int64
		E int
	}
	e := T{
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
		"A": "1",
		"B": "2",
		"C": "3",
		"D": "4",
		"E": "5",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(want)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotE, e) {
		t.Errorf("got %#v, want %#v", gotE, e)
	}
}

func TestFlattenUints(t *testing.T) {
	type T struct {
		A uint8
		B uint16
		C uint32
		D uint64
		E uint
	}
	e := T{
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
		"A": "1",
		"B": "2",
		"C": "3",
		"D": "4",
		"E": "5",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(want)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotE, e) {
		t.Errorf("got %#v, want %#v", gotE, e)
	}
}

func TestFlattenMaps(t *testing.T) {
	type T struct {
		Value map[string]int
	}
	e := T{
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
		"Value.one": "1",
		"Value.two": "2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(want)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotE, e) {
		t.Errorf("got %#v, want %#v", gotE, e)
	}
}

func TestFlattenSlices(t *testing.T) {
	type T struct {
		Value []int
	}
	e := T{
		Value: []int{1, 2, 3},
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"Value.0": "1",
		"Value.1": "2",
		"Value.2": "3",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(want)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotE, e) {
		t.Errorf("got %#v, want %#v", gotE, e)
	}
}

func TestFlattenArrays(t *testing.T) {
	type T struct {
		Value [3]int
	}
	e := T{
		Value: [3]int{1, 2, 3},
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"Value.0": "1",
		"Value.1": "2",
		"Value.2": "3",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(want)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotE, e) {
		t.Errorf("got %#v, want %#v", gotE, e)
	}
}

type stringer byte

func (stringer) String() string {
	return "stringer"
}

func TestFlattenStringers(t *testing.T) {
	type T struct {
		Value stringer
	}
	e := T{
		Value: 30,
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"Value": "stringer",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(want)); err == nil {
		t.Error("unexpectedly successful unflattening into stringer (want strconv error: parsing 'stringer': invalid syntax)")
	}
}

func TestFlattenArbitraryTypes(t *testing.T) {
	type T struct {
		Value complex64
	}
	e := T{
		Value: complex(17, 4),
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"Value": "(17+4i)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFlattenUnexportedFields(t *testing.T) {
	type T struct {
		value string
	}
	e := T{
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
	type T struct{}
	e := T{}

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

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(want)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotE, e) {
		t.Errorf("got %#v, want %#v", gotE, e)
	}
}

func TestFlattenDuration(t *testing.T) {
	type T struct {
		Value time.Duration
	}
	e := T{
		Value: 500 * time.Microsecond,
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"Value": "0.5",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(want)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotE, e) {
		t.Errorf("got %#v, want %#v", gotE, e)
	}
}

func TestFlattenPointers(t *testing.T) {
	type T struct {
		S *string
		I *int
	}

	s := "bar"
	i := 7
	e := T{
		S: &s,
		I: &i,
	}

	got := make(map[string]string)
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		got[k] = v
	})

	want := map[string]string{
		"S": "bar",
		"I": "7",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(want)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotE, e) {
		t.Errorf("got %#v, want %#v", gotE, e)
	}
}

func TestUnflattenExtraValues(t *testing.T) {
	type T struct {
		S string
		T map[string]int
		X struct{ Y string }
	}
	m := map[string]string{
		"A":     "a",
		"R":     "s",
		"S":     "s",
		"T.k1":  "3",
		"T.k2":  "4",
		"T":     "t",
		"U":     "5",
		"X Y":   "1",
		"X.Y":   "y",
		"X":     "x",
		"X.Y.Z": "4",
		"Z":     "7",
	}

	want := T{
		S: "s",
		T: map[string]int{"k1": 3, "k2": 4},
		X: struct{ Y string }{Y: "y"},
	}

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(m)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotE, want) {
		t.Errorf("got %#v, want %#v", gotE, want)
	}
}

func TestUnflatten_emptyMap(t *testing.T) {
	type T struct {
		A string
		B map[string]int
		C string
	}
	m := map[string]string{
		"A": "a",
		"C": "c",
	}

	want := T{
		A: "a",
		C: "c",
	}

	var gotE T
	if err := unflattenValue("", reflect.ValueOf(&gotE), reflect.TypeOf(&gotE), mapToKVs(m)); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotE, want) {
		t.Errorf("got %#v, want %#v", gotE, want)
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
