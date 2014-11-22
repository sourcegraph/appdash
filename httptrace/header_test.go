package httptrace

import (
	"net/http"
	"testing"

	"sourcegraph.com/sourcegraph/apptrace"
)

func TestSetSpanIDHeader(t *testing.T) {
	h := make(http.Header)
	SetSpanIDHeader(h, apptrace.SpanID{
		Trace: 100,
		Span:  150,
	})
	actual := h.Get("Span-ID")
	expected := "0000000000000064/0000000000000096"
	if actual != expected {
		t.Errorf("got %#v, want %#v", actual, expected)
	}
}

func TestGetSpanIDHeader(t *testing.T) {
	h := make(http.Header)
	h.Add("Span-ID", "0000000000000064/0000000000000096")
	id, err := GetSpanIDHeader(h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.Trace != 100 || id.Span != 150 {
		t.Errorf("unexpected span ID: %+v", id)
	}
}

func TestGetSpanIDHeaderMissing(t *testing.T) {
	h := make(http.Header)
	id, err := GetSpanIDHeader(h)
	if id != nil {
		t.Errorf("unexpected span ID: %+v", id)
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
