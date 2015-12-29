package httptrace

import (
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"sourcegraph.com/sourcegraph/appdash"
)

var _ appdash.Event = ClientEvent{}

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
	anns, err := appdash.MarshalEvent(e)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{
		"_schema:HTTPClient":                   "",
		"Client.Request.Headers.Connection":    "close",
		"Client.Request.Headers.Accept":        "application/json",
		"Client.Request.Headers.Authorization": "REDACTED",
		"Client.Request.Proto":                 "HTTP/1.1",
		"Client.Request.RemoteAddr":            "127.0.0.1",
		"Client.Request.Host":                  "example.com",
		"Client.Request.ContentLength":         "0",
		"Client.Request.Method":                "GET",
		"Client.Request.URI":                   "/foo",
		"Client.Response.StatusCode":           "200",
		"Client.Response.ContentLength":        "0",
		"Client.Send":                          "0001-01-01T00:00:00Z",
		"Client.Recv":                          "0001-01-01T00:00:00Z",
	}
	if !reflect.DeepEqual(anns.StringMap(), expected) {
		t.Errorf("got %#v, want %#v", anns.StringMap(), expected)
	}
}

func TestTransport(t *testing.T) {
	ms := appdash.NewMemoryStore()
	rec := appdash.NewRecorder(appdash.SpanID{1, 2, 3}, appdash.NewLocalCollector(ms))

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

	spanID, err := appdash.ParseSpanID(mt.req.Header.Get("Span-ID"))
	if err != nil {
		t.Fatal(err)
	}
	if want := (appdash.SpanID{1, spanID.Span, 2}); *spanID != want {
		t.Errorf("got Span-ID in header %+v, want %+v", *spanID, want)
	}

	trace, err := ms.Trace(1)
	if err != nil {
		t.Fatal(err)
	}

	var e ClientEvent
	if err := appdash.UnmarshalEvent(trace.Span.Annotations, &e); err != nil {
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

func TestCancelRequest(t *testing.T) {
	ms := appdash.NewMemoryStore()
	rec := appdash.NewRecorder(appdash.SpanID{1, 2, 3}, appdash.NewLocalCollector(ms))
	req, _ := http.NewRequest("GET", "http://example.com/foo", nil)
	transport := &Transport{
		Recorder: rec,
	}
	client := &http.Client{
		Timeout:   1 * time.Millisecond,
		Transport: transport,
	}

	resp, err := client.Do(req)

	expected := "Get http://example.com/foo: net/http: request canceled while waiting for connection"
	if err == nil || !strings.HasPrefix(err.Error(), expected) {
		t.Errorf("got %#v, want %s", err, expected)
	}
	if resp != nil {
		t.Errorf("got http.Response %#v, want nil", resp)
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
