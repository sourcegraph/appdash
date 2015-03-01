package traceapp

import (
	"fmt"
	"net/url"

	"github.com/gorilla/mux"
	"sourcegraph.com/sourcegraph/appdash"
)

// Traceapp's route names.
const (
	RootRoute             = "traceapp.root"               // route name for root
	TraceRoute            = "traceapp.trace"              // route name for a single trace page
	TraceSpanRoute        = "traceapp.trace.span"         // route name for a single trace sub-span page
	TraceProfileRoute     = "traceapp.trace.profile"      // route name for a JSON trace profile
	TraceSpanProfileRoute = "traceapp.trace.span.profile" // route name for a JSON trace sub-span profile
	TracesRoute           = "traceapp.traces"             // route name for traces page
)

// Router is a URL router for traceapp applications. It should be created via
// the NewRouter function.
type Router struct{ r *mux.Router }

// NewRouter creates a new URL router for a traceapp application.
func NewRouter(base *mux.Router) *Router {
	if base == nil {
		base = mux.NewRouter()
	}
	base.Path("/").Methods("GET").Name(RootRoute)
	base.Path("/traces/{Trace}").Methods("GET").Name(TraceRoute)
	base.Path("/traces/{Trace}/profile").Methods("GET").Name(TraceProfileRoute)
	base.Path("/traces/{Trace}/{Span}/profile").Methods("GET").Name(TraceSpanProfileRoute)
	base.Path("/traces/{Trace}/{Span}").Methods("GET").Name(TraceSpanRoute)
	base.Path("/traces").Methods("GET").Name(TracesRoute)
	return &Router{base}
}

// URLTo constructs a URL to a given route.
func (r *Router) URLTo(route string) (*url.URL, error) {
	rt := r.r.Get(route)
	if rt == nil {
		return nil, fmt.Errorf("no such route: %q", route)
	}
	return rt.URL()
}

// URLToTrace constructs a URL to a given trace by ID.
func (r *Router) URLToTrace(id appdash.ID) (*url.URL, error) {
	return r.r.Get(TraceRoute).URL("Trace", id.String())
}

// URLToTraceSpan constructs a URL to a sub-span in a trace.
func (r *Router) URLToTraceSpan(trace, span appdash.ID) (*url.URL, error) {
	return r.r.Get(TraceSpanRoute).URL("Trace", trace.String(), "Span", span.String())
}

// URLToTraceProfile constructs a URL to a trace's JSON profile.
func (r *Router) URLToTraceProfile(trace appdash.ID) (*url.URL, error) {
	return r.r.Get(TraceProfileRoute).URL("Trace", trace.String())
}

// URLToTraceSpanProfile constructs a URL to a sub-span's JSON profile in a
// trace.
func (r *Router) URLToTraceSpanProfile(trace, span appdash.ID) (*url.URL, error) {
	return r.r.Get(TraceSpanProfileRoute).URL("Trace", trace.String(), "Span", span.String())
}
