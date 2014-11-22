package httptrace

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	"sourcegraph.com/sourcegraph/apptrace"
)

var _ apptrace.Event = ClientEvent{}

func TestNewClientEvent(t *testing.T) {
	r := &http.Request{
		Host:          "example.com",
		Method:        "GET",
		URL:           &url.URL{Path: "/foo"},
		Proto:         "HTTP/1.1",
		RemoteAddr:    "127.0.0.1",
		ContentLength: 0,
		Header: http.Header{
			"Authorization": []string{"Basic seeecret"},
			"Accept":        []string{"application/json"},
		},
		Trailer: http.Header{
			"Authorization": []string{"Basic seeecret"},
			"Connection":    []string{"close"},
		},
	}
	e := NewClientEvent(r)
	e.Response.StatusCode = 200
	if e.Schema() != "HTTPClient" {
		t.Errorf("unexpected schema: %v", e.Schema())
	}
	anns, err := apptrace.MarshalEvent(e)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{
		"_schema:HTTPClient":            "",
		"Request.Headers.Connection":    "close",
		"Request.Headers.Accept":        "application/json",
		"Request.Headers.Authorization": "REDACTED",
		"Request.Proto":                 "HTTP/1.1",
		"Request.RemoteAddr":            "127.0.0.1",
		"Request.Host":                  "example.com",
		"Request.ContentLength":         "0",
		"Request.Method":                "GET",
		"Request.URI":                   "/foo",
		"Response.StatusCode":           "200",
		"Response.ContentLength":        "0",
		"ClientSend":                    "0001-01-01T00:00:00Z",
		"ClientRecv":                    "0001-01-01T00:00:00Z",
	}
	if !reflect.DeepEqual(anns.StringMap(), expected) {
		t.Errorf("got %#v, want %#v", anns.StringMap(), expected)
	}
}

func TestTransport(t *testing.T) {
	ms := apptrace.NewMemoryStore()
	rec := apptrace.NewRecorder(apptrace.SpanID{1, 2, 3}, apptrace.NewLocalCollector(ms))

	req, _ := http.NewRequest("GET", "http://example.com/foo", nil)
	req.Header.Set("X-Req-Header", "a")
	mt := &mockTransport{
		resp: &http.Response{
			StatusCode:    200,
			ContentLength: 123,
			Header:        http.Header{"X-Resp-Header": []string{"b"}},
		},
	}
	transport := &Transport{
		Recorder:  rec,
		Transport: mt,
	}

	_, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}

	spanID, err := apptrace.ParseSpanID(mt.req.Header.Get("Span-ID"))
	if err != nil {
		t.Fatal(err)
	}
	if want := (apptrace.SpanID{1, spanID.Span, 2}); *spanID != want {
		t.Errorf("got Span-ID in header %+v, want %+v", *spanID, want)
	}

	trace, err := ms.Trace(1)
	if err != nil {
		t.Fatal(err)
	}

	var e ClientEvent
	if err := apptrace.UnmarshalEvent(trace.Span.Annotations, &e); err != nil {
		t.Fatal(err)
	}

	wantEvent := ClientEvent{
		Request: RequestInfo{
			Method:  "GET",
			Proto:   "HTTP/1.1",
			URI:     "/foo",
			Host:    "example.com",
			Headers: map[string]string{"X-Req-Header": "a"},
		},
		Response: ResponseInfo{
			StatusCode:    200,
			ContentLength: 123,
			Headers:       map[string]string{"X-Resp-Header": "b"},
		},
	}
	delete(e.Request.Headers, "Span-Id")
	e.ClientSend = time.Time{}
	e.ClientRecv = time.Time{}
	if !reflect.DeepEqual(e, wantEvent) {
		t.Errorf("got ClientEvent %+v, want %+v", e, wantEvent)
	}
}

type mockTransport struct {
	req  *http.Request
	resp *http.Response
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.req = req
	return t.resp, nil
}
