package traceapp

import (
	"fmt"
	"net/url"

	"github.com/gorilla/mux"
	"sourcegraph.com/sourcegraph/apptrace"
)

const (
	RootRoute   = "traceapp.root"   // route name for root
	TraceRoute  = "traceapp.trace"  // route name for a single trace page
	TracesRoute = "traceapp.traces" // route name for traces page
)

type Router struct{ r *mux.Router }

// NewRouter creates a new URL router for a traceapp application.
func NewRouter(base *mux.Router) *Router {
	if base == nil {
		base = mux.NewRouter()
	}
	base.Path("/").Methods("GET").Name(RootRoute)
	base.Path("/traces/{Trace}").Methods("GET").Name(TraceRoute)
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
func (r *Router) URLToTrace(id apptrace.ID) (*url.URL, error) {
	return r.r.Get(TraceRoute).URL("Trace", id.String())
}
