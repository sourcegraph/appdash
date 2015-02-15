package httptrace

import (
	"net/http"
	"testing"

	"sourcegraph.com/sourcegraph/appdash"
)

func TestSetSpanIDHeader(t *testing.T) {
	h := make(http.Header)
	SetSpanIDHeader(h, appdash.SpanID{
		Trace: 100,
		Span:  150,
	})
	actual := h.Get("Span-ID")
	expected := "0000000000000064/0000000000000096"
	if actual != expected {
		t.Errorf("got %#v, want %#v", actual, expected)
	}
}

func TestGetSpanID_hasSpanID(t *testing.T) {
	h := make(http.Header)
	h.Add("Span-ID", "0000000000000064/0000000000000096")
	id, err := GetSpanID(h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.Trace != 100 || id.Span != 150 {
		t.Errorf("unexpected span ID: %+v", id)
	}
}

func TestGetSpanID_hasParentSpanID(t *testing.T) {
	h := make(http.Header)
	h.Add("Parent-Span-ID", "0000000000000064/0000000000000096")
	id, err := GetSpanID(h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.Trace != 100 || id.Parent != 150 {
		t.Errorf("unexpected span ID: %+v", id)
	}
	if id.Span == 150 {
		t.Errorf("unexpected span ID: %+v", id)
	}
}

func TestGetSpanID_hasNoSpanID(t *testing.T) {
	h := make(http.Header)
	id, err := GetSpanID(h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil {
		t.Fatal("got nil ID, expected a new root span ID")
	}
	if id.Trace == 0 || id.Span == 0 {
		t.Errorf("unexpected span ID: %+v", id)
	}
	if id.Parent != 0 {
		t.Errorf("unexpected span ID nonzero parent: %+v", id)
	}
}

func TestGetSpanID_hasSpanIDAndParentSpanID(t *testing.T) {
	// This should never happen, but just make sure we don't fail (or
	// accidentally change the behavior).
	h := make(http.Header)
	h.Add("Span-ID", "0000000000000064/0000000000000096")
	h.Add("Parent-Span-ID", "0000000000000032/0000000000000048")
	id, err := GetSpanID(h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.Trace != 100 || id.Span != 150 {
		t.Errorf("unexpected span ID: %+v", id)
	}
}
