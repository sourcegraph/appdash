// Package traceapp implements the Appdash web UI.
//
// The web UI can be effectively launched using the appdash command (see
// cmd/appdash) or via embedding this package within your app.
//
// Templates and other resources needed by this package to render the UI are
// built into the program using go-bindata, so you still get to have single
// binary deployment.
//
// For an example of embedding the Appdash web UI within your own application
// via the traceapp package, see the examples/cmd/webapp example.
package traceapp

import (
	"encoding/json"
	htmpl "html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"sort"
	"sync"

	"github.com/gorilla/mux"

	"sourcegraph.com/sourcegraph/appdash"
)

// App is an HTTP application handler that also exposes methods for
// constructing URL routes.
type App struct {
	*Router

	Store   appdash.Store
	Queryer appdash.Queryer

	tmplLock sync.Mutex
	tmpls    map[string]*htmpl.Template
}

// New creates a new application handler. If r is nil, a new router is
// created.
func New(r *Router) *App {
	if r == nil {
		r = NewRouter(nil)
	}

	app := &App{Router: r}

	r.r.Get(RootRoute).Handler(handlerFunc(app.serveRoot))
	r.r.Get(TraceRoute).Handler(handlerFunc(app.serveTrace))
	r.r.Get(TraceSpanRoute).Handler(handlerFunc(app.serveTrace))
	r.r.Get(TraceProfileRoute).Handler(handlerFunc(app.serveTrace))
	r.r.Get(TraceSpanProfileRoute).Handler(handlerFunc(app.serveTrace))
	r.r.Get(TraceUploadRoute).Handler(handlerFunc(app.serveTraceUpload))
	r.r.Get(TracesRoute).Handler(handlerFunc(app.serveTraces))

	return app
}

// ServeHTTP implements http.Handler.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.Router.r.ServeHTTP(w, r)
}

func (a *App) serveRoot(w http.ResponseWriter, r *http.Request) error {
	return a.renderTemplate(w, r, "root.html", http.StatusOK, &struct {
		TemplateCommon
	}{})
}

func (a *App) serveTrace(w http.ResponseWriter, r *http.Request) error {
	v := mux.Vars(r)

	traceID, err := appdash.ParseID(v["Trace"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	trace, err := a.Store.Trace(traceID)
	if err != nil {
		if err == appdash.ErrTraceNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return nil
		}
		return err
	}

	// Get sub-span if the Span route var is present.
	if spanIDStr := v["Span"]; spanIDStr != "" {
		spanID, err := appdash.ParseID(spanIDStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return err
		}
		trace = trace.FindSpan(spanID)
		if trace == nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return nil
		}
	}

	// We could use a separate handler for this, but as we need the above to
	// determine the correct trace (or therein sub-trace), we just handle any
	// JSON profile requests here.
	if path.Base(r.URL.Path) == "profile" {
		return a.profile(trace, w)
	}

	visData, err := a.d3timeline(trace)
	if err != nil {
		return err
	}

	// Determine the profile URL.
	var profile *url.URL
	if trace.ID.Parent == 0 {
		profile, err = a.Router.URLToTraceProfile(trace.Span.ID.Trace)
	} else {
		profile, err = a.Router.URLToTraceSpanProfile(trace.Span.ID.Trace, trace.Span.ID.Span)
	}
	if err != nil {
		return err
	}

	return a.renderTemplate(w, r, "trace.html", http.StatusOK, &struct {
		TemplateCommon
		Trace      *appdash.Trace
		VisData    []timelineItem
		ProfileURL string
	}{
		Trace:      trace,
		VisData:    visData,
		ProfileURL: profile.String(),
	})
}

func (a *App) serveTraces(w http.ResponseWriter, r *http.Request) error {
	traces, err := a.Queryer.Traces()
	if err != nil {
		return err
	}

	// Sort the traces by ID to ensure that the display order doesn't change upon
	// multiple page reloads if Queryer.Traces is e.g. backed by a map (which has
	// a random iteration order).
	sort.Sort(tracesByID(traces))

	return a.renderTemplate(w, r, "traces.html", http.StatusOK, &struct {
		TemplateCommon
		Traces []*appdash.Trace
	}{
		Traces: traces,
	})
}

func (a *App) serveTraceUpload(w http.ResponseWriter, r *http.Request) error {
	// Read the uploaded JSON trace data.
	defer r.Body.Close()
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	// Unmarshal the trace.
	var traces []*appdash.Trace
	err = json.Unmarshal(data, &traces)
	if err != nil {
		return err
	}

	// Collect the unmarshaled traces, ignoring any previously existing ones (i.e.
	// ones that would collide / be merged together).
	for _, trace := range traces {
		_, err = a.Store.Trace(trace.Span.ID.Trace)
		if err != appdash.ErrTraceNotFound {
			// The trace collides with an existing trace, ignore it.
			continue
		}

		// Collect the trace (store it for later viewing).
		if err = collectTrace(a.Store, trace); err != nil {
			return err
		}
	}
	return nil
}
