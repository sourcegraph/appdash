package appdash

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	influxDBServer "github.com/influxdata/influxdb/cmd/influxd/run"
)

const (
	clientEventKey    string = schemaPrefix + clientEventSchema
	clientEventSchema string = "HTTPClient"
	serverEventKey    string = schemaPrefix + serverEventSchema
	serverEventSchema string = "HTTPServer"
	spanNameSchema    string = "name"
)

func TestMergeSchemasField(t *testing.T) {
	cases := []struct {
		NewField string
		OldField string
		Want     string
	}{
		{NewField: "", OldField: "", Want: ""},
		{NewField: "HTTPClient", OldField: "", Want: "HTTPClient"},
		{NewField: "", OldField: "name", Want: "name"},
		{NewField: "HTTPClient", OldField: "name", Want: "HTTPClient,name"},
		{NewField: "HTTPServer", OldField: "HTTPClient,name", Want: "HTTPServer,HTTPClient,name"},
	}
	for i, c := range cases {
		got, err := mergeSchemasField(c.NewField, c.OldField)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got = sortSchemas(got)
		want := sortSchemas(c.Want)
		if got != want {
			t.Fatalf("case #%d - got: %v, want: %v", i, got, c.Want)
		}
	}
}

func TestSchemasFromAnnotations(t *testing.T) {
	anns := []Annotation{
		Annotation{Key: schemaPrefix + "HTTPClient"},
		Annotation{Key: schemaPrefix + "HTTPServer"},
		Annotation{Key: schemaPrefix + "name"},
	}
	got := sortSchemas(schemasFromAnnotations(anns))
	want := sortSchemas("HTTPClient,HTTPServer,name")
	if got != want {
		t.Fatalf("got: %v, want: %v", got, want)
	}
}

func TestInfluxDBStore(t *testing.T) {
	store := newStore(t)
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	trace := Trace{
		Span: Span{
			ID: SpanID{1, 2, ID(0)},
			Annotations: Annotations{
				Annotation{Key: "Name", Value: []byte("/")},
				Annotation{Key: "Server.Request.Method", Value: []byte("GET")},
				Annotation{Key: clientEventKey, Value: []byte("")},
				Annotation{Key: serverEventKey, Value: []byte("")},
			},
		},
		Sub: []*Trace{
			&Trace{
				Span: Span{
					ID: SpanID{Trace: 1, Span: 11, Parent: 1},
					Annotations: Annotations{
						Annotation{Key: "Name", Value: []byte("localhost:8699/endpoint")},
						Annotation{Key: "Server.Request.Method", Value: []byte("GET")},
						Annotation{Key: clientEventKey, Value: []byte("")},
						Annotation{Key: serverEventKey, Value: []byte("")},
					},
				},
			},
		},
	}

	// Collect root span.
	if err := store.Collect(trace.Span.ID, trace.Span.Annotations...); err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}

	// Collect first child span.
	if err := store.Collect(trace.Sub[0].Span.ID, trace.Sub[0].Span.Annotations...); err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}

	// Find one trace.
	savedTrace, err := store.Trace(trace.Span.ID.Trace)
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}
	if savedTrace == nil {
		t.Fatalf("expected trace, got nil")
	}
	// Non-deterministic keys:
	// - "time" added by InfluxDB.
	// - "schemas" see: `schemasFieldName` within `InfluxDBStore.Collect(...)`.
	keys := []string{"time", "schemas"}

	// Using extendAnnotations in order to create a trace which includes those
	// non-deterministic annotations with keys included in `fields` and values
	// taken from `savedTrace`.
	wantTrace := extendAnnotations(trace, *savedTrace, keys)

	sortAnnotations(*savedTrace, wantTrace)
	if !reflect.DeepEqual(*savedTrace, wantTrace) {
		t.Fatalf("got: %v, want: %v", savedTrace, wantTrace)
	}

	// Find many traces.
	traces, err := store.Traces()
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}
	if len(traces) != 1 {
		t.Fatalf("unexpected quantity of traces, want: %v, got: %v", 1, len(traces))
	}
	sortAnnotations(*traces[0])
	if !reflect.DeepEqual(*traces[0], wantTrace) {
		t.Fatalf("got: %v, want: %v", traces[0], wantTrace)
	}
}

func newStore(t *testing.T) *InfluxDBStore {
	conf, err := influxDBServer.NewDemoConfig()
	if err != nil {
		t.Fatalf("failed to create influxdb config, error: %v", err)
	}
	conf.HTTPD.AuthEnabled = true
	user := InfluxDBAdminUser{Username: "demo", Password: "demo"}
	store, err := NewInfluxDBStore(InfluxDBStoreConfig{
		AdminUser: user,
		Server:    conf,
		BuildInfo: &influxDBServer.BuildInfo{},
		Mode:      testMode,
	})
	if err != nil {
		t.Fatalf("failed to create influxdb store, error: %v", err)
	}
	return store
}

// extendAnnotations creates & returns a new Trace which is a copy of `dst`.
// Trace returned has `annotations` copied from `src` but only those
// with keys included on `keys`.
func extendAnnotations(dst, src Trace, keys []string) Trace {
	t := dst
	for _, k := range keys {
		t.Span.Annotations = append(t.Span.Annotations, Annotation{
			Key:   k,
			Value: src.Span.Annotations.get(k),
		})
	}
	for i, sub := range t.Sub {
		for _, k := range keys {
			sub.Span.Annotations = append(sub.Span.Annotations, Annotation{
				Key:   k,
				Value: src.Sub[i].Span.Annotations.get(k),
			})
		}
	}
	return t
}

// sortSchemas sorts schemas(strings) within `s` which is
// a set of schemas separated by `schemasFieldSeparator`.
func sortSchemas(s string) string {
	schemas := strings.Split(s, schemasFieldSeparator)
	sort.Sort(bySchemaText(schemas))
	return strings.Join(schemas, schemasFieldSeparator)
}

func sortAnnotations(traces ...Trace) {
	for _, t := range traces {
		sort.Sort(annotations(t.Span.Annotations))
		for _, s := range t.Sub {
			sort.Sort(annotations(s.Span.Annotations))
		}
	}
}

type bySchemaText []string

func (bs bySchemaText) Len() int           { return len(bs) }
func (bs bySchemaText) Swap(i, j int)      { bs[i], bs[j] = bs[j], bs[i] }
func (bs bySchemaText) Less(i, j int) bool { return bs[i] < bs[j] }
