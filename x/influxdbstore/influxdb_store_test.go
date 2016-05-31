package influxdbstore

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"sourcegraph.com/sourcegraph/appdash"
)

func TestAddChildren(t *testing.T) {
	root := &appdash.Trace{Span: appdash.Span{ID: appdash.SpanID{1, 100, 0}}}
	want := &appdash.Trace{
		Span: root.Span,
		Sub: []*appdash.Trace{
			&appdash.Trace{
				Span: appdash.Span{ID: appdash.SpanID{1, 101, 100}},
				Sub: []*appdash.Trace{
					&appdash.Trace{
						Span: appdash.Span{ID: appdash.SpanID{1, 1011, 101}},
						Sub: []*appdash.Trace{
							&appdash.Trace{
								Span: appdash.Span{ID: appdash.SpanID{1, 10111, 1011}},
							},
							&appdash.Trace{
								Span: appdash.Span{ID: appdash.SpanID{1, 10112, 1011}},
							},
						},
					},
					&appdash.Trace{
						Span: appdash.Span{ID: appdash.SpanID{1, 1012, 101}},
					},
				},
			},
			&appdash.Trace{
				Span: appdash.Span{ID: appdash.SpanID{1, 102, 100}},
				Sub: []*appdash.Trace{
					&appdash.Trace{
						Span: appdash.Span{ID: appdash.SpanID{1, 1021, 102}},
						Sub: []*appdash.Trace{
							&appdash.Trace{
								Span: appdash.Span{ID: appdash.SpanID{1, 10211, 1021}},
							},
						},
					},
				},
			},
		},
	}
	var (
		children      []*appdash.Trace
		sortSubTraces func(root *appdash.Trace)
		subTraces     func(root *appdash.Trace, traces []*appdash.Trace) []*appdash.Trace
	)
	subTraces = func(root *appdash.Trace, traces []*appdash.Trace) []*appdash.Trace {
		traces = append(traces, root.Sub...)
		for _, sub := range root.Sub {
			subTraces(sub, traces)
		}
		return traces
	}
	sortSubTraces = func(root *appdash.Trace) {
		sort.Sort(tracesByIDSpan(root.Sub))
		for _, sub := range root.Sub {
			sortSubTraces(sub)
		}
	}
	if err := addChildren(root, subTraces(want, children)); err != nil {
		t.Fatal(err)
	}
	sortSubTraces(root)
	sortSubTraces(want)
	if !reflect.DeepEqual(root, want) {
		t.Fatalf("got: %v, want: %v", root, want)
	}
}

func TestSchemasFromAnnotations(t *testing.T) {
	anns := []appdash.Annotation{
		appdash.Annotation{Key: appdash.SchemaPrefix + "HTTPClient"},
		appdash.Annotation{Key: appdash.SchemaPrefix + "HTTPServer"},
		appdash.Annotation{Key: appdash.SchemaPrefix + "name"},
	}
	got := sortSchemas(schemasFromAnnotations(anns))
	want := sortSchemas("HTTPClient,HTTPServer,name")
	if got != want {
		t.Fatalf("got: %v, want: %v", got, want)
	}
}

func TestFindTraceParent(t *testing.T) {
	trace := appdash.Trace{
		Span: appdash.Span{
			ID: appdash.SpanID{Trace: 1, Span: 100, Parent: 0},
		},
		Sub: []*appdash.Trace{
			&appdash.Trace{
				Span: appdash.Span{
					ID: appdash.SpanID{Trace: 1, Span: 11, Parent: 100},
				},
				Sub: []*appdash.Trace{
					&appdash.Trace{
						Span: appdash.Span{
							ID: appdash.SpanID{Trace: 1, Span: 111, Parent: 11},
						},
						Sub: []*appdash.Trace{
							&appdash.Trace{
								Span: appdash.Span{
									ID: appdash.SpanID{Trace: 1, Span: 1111, Parent: 111},
								},
							},
						},
					},
					&appdash.Trace{
						Span: appdash.Span{
							ID: appdash.SpanID{Trace: 1, Span: 112, Parent: 11},
						},
						Sub: []*appdash.Trace{
							&appdash.Trace{
								Span: appdash.Span{
									ID: appdash.SpanID{Trace: 1, Span: 1112, Parent: 112},
								},
							},
						},
					},
				},
			},
		},
	}
	cases := []struct {
		Parent *appdash.Trace
		Child  *appdash.Trace
	}{
		{nil, &trace},
		{nil, &appdash.Trace{}},
		{&trace, trace.Sub[0]},
		{trace.Sub[0], trace.Sub[0].Sub[0]},
		{trace.Sub[0], trace.Sub[0].Sub[1]},
		{trace.Sub[0].Sub[0], trace.Sub[0].Sub[0].Sub[0]},
		{trace.Sub[0].Sub[1], trace.Sub[0].Sub[1].Sub[0]},
	}
	for i, c := range cases {
		got := findTraceParent(&trace, c.Child)
		if got != c.Parent {
			t.Fatalf("case: %d - got: %v, want: %v", i, got, c.Parent)
		}
	}
}

func TestInfluxDBStore(t *testing.T) {
	store, err := newTestInfluxDBStore()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	traces := []*appdash.Trace{
		&appdash.Trace{
			Span: appdash.Span{
				ID: appdash.SpanID{1, 100, 0},
				Annotations: appdash.Annotations{
					appdash.Annotation{Key: "Name", Value: []byte("/")},
					appdash.Annotation{Key: "_schema:name"},
				},
			},
			Sub: []*appdash.Trace{
				&appdash.Trace{
					Span: appdash.Span{
						ID: appdash.SpanID{Trace: 1, Span: 11, Parent: 100},
						Annotations: appdash.Annotations{
							appdash.Annotation{Key: "Name", Value: []byte("localhost:8699/endpoint")},
							appdash.Annotation{Key: "_schema:name"},
						},
					},
					Sub: []*appdash.Trace{
						&appdash.Trace{
							Span: appdash.Span{
								ID: appdash.SpanID{Trace: 1, Span: 111, Parent: 11},
								Annotations: appdash.Annotations{
									appdash.Annotation{Key: "Name", Value: []byte("localhost:8699/sub1")},
									appdash.Annotation{Key: "_schema:name"},
								},
							},
							Sub: []*appdash.Trace{
								&appdash.Trace{
									Span: appdash.Span{
										ID: appdash.SpanID{Trace: 1, Span: 1111, Parent: 111},
										Annotations: appdash.Annotations{
											appdash.Annotation{Key: "Name", Value: []byte("localhost:8699/sub2")},
											appdash.Annotation{Key: "_schema:name"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		&appdash.Trace{
			Span: appdash.Span{
				ID: appdash.SpanID{2, 200, 0},
				Annotations: appdash.Annotations{
					appdash.Annotation{Key: "Name", Value: []byte("/")},
					appdash.Annotation{Key: "_schema:name"},
				},
			},
		},
	}

	var (
		keys           = []string{"time", "schemas"} // InfluxDB related annotations keys.
		mustCollect    func(trace *appdash.Trace)
		mustCollectAll func(trace *appdash.Trace)
		tracesMap      = make(map[appdash.ID]*appdash.Trace, 0) // Trace ID -> Trace.
	)

	mustCollect = func(trace *appdash.Trace) {
		if err := store.Collect(trace.Span.ID, trace.Span.Annotations...); err != nil {
			t.Fatalf("unexpected error: %+v", err)
		}
	}
	mustCollectAll = func(trace *appdash.Trace) {
		for _, sub := range trace.Sub {
			mustCollect(sub)
			mustCollectAll(sub)
		}
	}
	for _, trace := range traces {
		tracesMap[trace.ID.Trace] = trace
	}

	// InfluxDBStore.Collect(...) tests.
	for _, trace := range traces {
		mustCollect(trace)
		mustCollectAll(trace)
	}

	mustBeEqual := func(gotTrace, want *appdash.Trace) {
		removeInfluxDBAnnotations(gotTrace, keys)
		sortAnnotations(*gotTrace, *want)
		if !reflect.DeepEqual(gotTrace, want) {
			t.Fatalf("got: %v, want: %v", gotTrace, want)
		}
	}

	if err := store.flush(); err != nil {
		t.Fatalf("flush:", err)
	}

	// InfluxDBStore.Trace(...) tests.
	for _, trace := range traces {
		gotTrace, err := store.Trace(trace.ID.Trace)
		if err != nil {
			t.Fatalf("unexpected error: %+v", err)
		}
		if t == nil {
			t.Fatalf("expected a trace, got nil")
		}
		want, found := tracesMap[gotTrace.ID.Trace]
		if !found {
			t.Fatal("trace not found")
		}
		mustBeEqual(gotTrace, want)
	}

	// InfluxDBStore.Traces(...) tests.
	gotTraces, err := store.Traces(appdash.TracesOpts{})
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}
	if len(gotTraces) != len(traces) {
		t.Fatalf("unexpected quantity of traces, got: %v, want: %v", len(gotTraces), len(traces))
	}
	for _, gotTrace := range gotTraces {
		want, found := tracesMap[gotTrace.ID.Trace]
		if !found {
			t.Fatal("trace not found")
		}
		mustBeEqual(gotTrace, want)
	}
}

func benchmarkInfluxDBStoreCollect(b *testing.B, n int) {
	b.StopTimer()
	store, err := newTestInfluxDBStore()
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			b.Fatal(err)
		}
	}()
	b.StartTimer()
	var x appdash.ID
	for n := 0; n < b.N; n++ {
		for c := 0; c < n; c++ {
			x++
			spanID := appdash.SpanID{x, x + 1, 0}
			anns := []appdash.Annotations{
				appdash.Annotations{
					appdash.Annotation{Key: "Server.Request.Method", Value: []byte("GET")},
					appdash.Annotation{Key: "Server.Request.Headers.User-Agent", Value: []byte("Go-http-client/1.1")},
					appdash.Annotation{Key: "_schema:HTTPServer", Value: []byte("")},
				},
				appdash.Annotations{
					appdash.Annotation{Key: "Name", Value: []byte("/")},
				},
				appdash.Annotations{
					appdash.Annotation{Key: "Client.Response.Headers.Content-Type", Value: []byte("text/plain; charset=utf-8")},
					appdash.Annotation{Key: "Client.Response.Headers.Content-Length", Value: []byte("16")},
					appdash.Annotation{Key: "_schema:HTTPClient", Value: []byte("")},
				},
			}
			for ann := 0; ann < len(anns); ann++ {
				if err := store.Collect(spanID, anns[ann]...); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
	b.StopTimer()
}

func BenchmarkInfluxDBStoreCollect100(b *testing.B) {
	benchmarkInfluxDBStoreCollect(b, 100)
}

func BenchmarkInfluxDBStoreCollect250(b *testing.B) {
	benchmarkInfluxDBStoreCollect(b, 250)
}

func BenchmarkInfluxDBStoreCollect1000(b *testing.B) {
	benchmarkInfluxDBStoreCollect(b, 1000)
}

func BenchmarkInfluxDBStoreCollectParallel(b *testing.B) {
	b.StopTimer()
	store, err := newTestInfluxDBStore()
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			b.Fatal(err)
		}
	}()
	b.StartTimer()
	var x appdash.ID
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			x++
			spanID := appdash.SpanID{x, x + 1, 0}
			anns := []appdash.Annotations{
				appdash.Annotations{
					appdash.Annotation{Key: "Server.Request.Method", Value: []byte("GET")},
					appdash.Annotation{Key: "Server.Request.Headers.User-Agent", Value: []byte("Go-http-client/1.1")},
					appdash.Annotation{Key: "_schema:HTTPServer", Value: []byte("")},
				},
				appdash.Annotations{
					appdash.Annotation{Key: "Name", Value: []byte("/")},
				},
				appdash.Annotations{
					appdash.Annotation{Key: "Client.Response.Headers.Content-Type", Value: []byte("text/plain; charset=utf-8")},
					appdash.Annotation{Key: "Client.Response.Headers.Content-Length", Value: []byte("16")},
					appdash.Annotation{Key: "_schema:HTTPClient", Value: []byte("")},
				},
			}
			for ann := 0; ann < len(anns); ann++ {
				if err := store.Collect(spanID, anns[ann]...); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	b.StopTimer()
}

func benchmarkInfluxDBStoreTrace(b *testing.B, n int) {
	b.StopTimer()
	store, err := newTestInfluxDBStore()
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			b.Fatal(err)
		}
	}()
	traces, err := benchmarkInfluxDBStoreCreateTraces(store, n)
	if err != nil {
		b.Fatal(err)
	}
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		for _, trace := range traces {
			if _, err := store.Trace(trace.ID.Trace); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.StopTimer()
}

func BenchmarkInfluxDBStoreTrace100(b *testing.B) {
	benchmarkInfluxDBStoreTrace(b, 100)
}

func BenchmarkInfluxDBStoreTrace250(b *testing.B) {
	benchmarkInfluxDBStoreTrace(b, 250)
}

func BenchmarkInfluxDBStoreTrace1000(b *testing.B) {
	benchmarkInfluxDBStoreTrace(b, 1000)
}

func benchmarkInfluxDBStoreTraces(b *testing.B, n int) {
	b.StopTimer()
	store, err := newTestInfluxDBStore()
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			b.Fatal(err)
		}
	}()
	if _, err := benchmarkInfluxDBStoreCreateTraces(store, n); err != nil {
		b.Fatal(err)
	}
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		if _, err := store.Traces(appdash.TracesOpts{}); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
}

func BenchmarkInfluxDBStoreTracesDefaultPerPage(b *testing.B) {
	benchmarkInfluxDBStoreTraces(b, defaultTracesPerPage)
}

func benchmarkInfluxDBStoreCreateTraces(store *InfluxDBStore, n int) ([]*appdash.Trace, error) {
	var (
		mustCollect    func(trace *appdash.Trace) error
		mustCollectAll func(trace *appdash.Trace) error
	)
	mustCollect = func(trace *appdash.Trace) error {
		if err := store.Collect(trace.Span.ID, trace.Span.Annotations...); err != nil {
			return err
		}
		return nil
	}
	mustCollectAll = func(trace *appdash.Trace) error {
		if err := mustCollect(trace); err != nil {
			return err
		}
		for _, sub := range trace.Sub {
			if err := mustCollect(sub); err != nil {
				return err
			}
			if err := mustCollectAll(sub); err != nil {
				return err
			}
		}
		return nil
	}
	var (
		// Initial ID's for traces & sub-traces.
		x      = appdash.ID(0)     // Root.
		s0     = appdash.ID(n)     // Sub 0.
		s1     = appdash.ID(n * 2) // Sub 1.
		s2     = appdash.ID(n * 3) // Sub 2.
		s3     = appdash.ID(n * 4) // Sub 3.
		ids    = []*appdash.ID{&x, &s0, &s1, &s2, &s3}
		traces []*appdash.Trace
	)
	for c := 0; c < n; c++ {
		for _, id := range ids {
			*id++
		}
		trace := appdash.Trace{
			Span: appdash.Span{
				ID: appdash.SpanID{x, s0, 0},
				Annotations: []appdash.Annotation{
					appdash.Annotation{Key: "Server.Request.Method", Value: []byte("GET")},
					appdash.Annotation{Key: "Server.Request.Headers.User-Agent", Value: []byte("Go-http-client/1.1")},
					appdash.Annotation{Key: "_schema:HTTPServer", Value: []byte("")},
					appdash.Annotation{Key: "Name", Value: []byte("/")},
					appdash.Annotation{Key: "Client.Response.Headers.Content-Type", Value: []byte("text/plain; charset=utf-8")},
					appdash.Annotation{Key: "Client.Response.Headers.Content-Length", Value: []byte("16")},
					appdash.Annotation{Key: "_schema:HTTPClient", Value: []byte("")},
				},
			},
			Sub: []*appdash.Trace{
				&appdash.Trace{
					Span: appdash.Span{
						ID: appdash.SpanID{Trace: x, Span: s1, Parent: s0},
						Annotations: appdash.Annotations{
							appdash.Annotation{Key: "Name", Value: []byte("localhost:8699/endpoint")},
							appdash.Annotation{Key: "Server.Request.Method", Value: []byte("GET")},
							appdash.Annotation{Key: "_schema:HTTPClient", Value: []byte("")},
							appdash.Annotation{Key: "_schema:HTTPServer", Value: []byte("")},
						},
					},
					Sub: []*appdash.Trace{
						&appdash.Trace{
							Span: appdash.Span{
								ID: appdash.SpanID{Trace: x, Span: s2, Parent: s1},
								Annotations: appdash.Annotations{
									appdash.Annotation{Key: "Name", Value: []byte("localhost:8699/sub1")},
									appdash.Annotation{Key: "Server.Request.Method", Value: []byte("GET")},
									appdash.Annotation{Key: "_schema:HTTPClient", Value: []byte("")},
									appdash.Annotation{Key: "_schema:HTTPServer", Value: []byte("")},
								},
							},
							Sub: []*appdash.Trace{
								&appdash.Trace{
									Span: appdash.Span{
										ID: appdash.SpanID{Trace: x, Span: s3, Parent: s2},
										Annotations: appdash.Annotations{
											appdash.Annotation{Key: "Name", Value: []byte("localhost:8699/sub2")},
											appdash.Annotation{Key: "Server.Request.Method", Value: []byte("GET")},
											appdash.Annotation{Key: "_schema:HTTPClient", Value: []byte("")},
											appdash.Annotation{Key: "_schema:HTTPServer", Value: []byte("")},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		if err := mustCollectAll(&trace); err != nil {
			return nil, err
		}
		traces = append(traces, &trace)
	}
	return traces, nil
}

func newTestInfluxDBStore() (*InfluxDBStore, error) {
	// Create a default InfluxDB configuration.
	conf, err := NewInfluxDBConfig()
	if err != nil {
		return nil, err
	}

	// Enable InfluxDB server HTTP basic auth.
	conf.Server.HTTPD.AuthEnabled = true
	conf.AdminUser = InfluxDBAdminUser{
		Username: "demo",
		Password: "demo",
	}

	// Disable metrics reporting because we're just a test.
	conf.Server.ReportingDisabled = true

	// Switch mode to testMode.
	conf.Mode = testMode

	store, err := NewInfluxDBStore(conf)
	if err != nil {
		return nil, err
	}
	return store, nil
}

// removeInfluxDBAnnotations removes annotations from `root` and it's subtraces; only those annotations that have as key present on `keys` will be removed.
func removeInfluxDBAnnotations(root *appdash.Trace, keys []string) {
	var (
		walk     func(root *appdash.Trace)
		removeFn func(trace *appdash.Trace, keys []string)
	)
	removeFn = func(trace *appdash.Trace, keys []string) {
		for i := len(trace.Annotations) - 1; i >= 0; i-- {
			for _, k := range keys {
				if trace.Annotations[i].Key == k {
					trace.Annotations = append(trace.Annotations[:i], trace.Annotations[i+1:]...)
					break
				}
			}
		}
	}
	walk = func(root *appdash.Trace) {
		removeFn(root, keys)
		for _, sub := range root.Sub {
			removeFn(sub, keys)
			walk(sub)
		}
	}
	walk(root)
}

// sortSchemas sorts schemas(strings) within `s` which is
// a set of schemas separated by `schemasFieldSeparator`.
func sortSchemas(s string) string {
	schemas := strings.Split(s, schemasFieldSeparator)
	sort.Sort(bySchemaText(schemas))
	return strings.Join(schemas, schemasFieldSeparator)
}

func sortAnnotations(traces ...appdash.Trace) {
	var walk func(t *appdash.Trace)
	walk = func(t *appdash.Trace) {
		sort.Sort(annotations(t.Span.Annotations))
		for _, s := range t.Sub {
			sort.Sort(annotations(s.Span.Annotations))
			walk(s)
		}
	}
	for _, t := range traces {
		walk(&t)
	}
}

type bySchemaText []string

func (bs bySchemaText) Len() int           { return len(bs) }
func (bs bySchemaText) Swap(i, j int)      { bs[i], bs[j] = bs[j], bs[i] }
func (bs bySchemaText) Less(i, j int) bool { return bs[i] < bs[j] }

type tracesByIDSpan []*appdash.Trace

func (t tracesByIDSpan) Len() int           { return len(t) }
func (t tracesByIDSpan) Less(i, j int) bool { return t[i].Span.ID.Span < t[j].Span.ID.Span }
func (t tracesByIDSpan) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }

type annotations appdash.Annotations

func (a annotations) Len() int           { return len(a) }
func (a annotations) Less(i, j int) bool { return a[i].Key < a[j].Key }
func (a annotations) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
