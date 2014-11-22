package httptrace

import (
	"net/http"

	"sourcegraph.com/sourcegraph/apptrace"
)

const (
	// HeaderSpanID is the name of the HTTP header by which the trace
	// and span IDs are passed along.
	HeaderSpanID = "Span-ID"
)

// SetSpanIDHeader sets the Span-ID header.
func SetSpanIDHeader(h http.Header, e apptrace.SpanID) {
	h.Set(HeaderSpanID, e.String())
}

// GetSpanIDHeader returns the SpanID in the headers, nil if no
// Span-ID was provided, or an error if the value was unparseable.
func GetSpanIDHeader(h http.Header) (*apptrace.SpanID, error) {
	s := h.Get(HeaderSpanID)
	if s == "" {
		return nil, nil
	}
	return apptrace.ParseSpanID(s)
}
