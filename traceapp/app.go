package traceapp

import (
	htmpl "html/template"
	"net/http"
	"net/url"
	"path"
	"sync"

	"github.com/gorilla/mux"

	"sourcegraph.com/sourcegraph/apptrace"
)

// App is an HTTP application handler that also exposes methods for
// constructing URL routes.
type App struct {
	*Router

	Store   apptrace.Store
	Queryer apptrace.Queryer

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

	traceID, err := apptrace.ParseID(v["Trace"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	trace, err := a.Store.Trace(traceID)
	if err != nil {
		if err == apptrace.ErrTraceNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return nil
		}
		return err
	}

	// Get sub-span if the Span route var is present.
	if spanIDStr := v["Span"]; spanIDStr != "" {
		spanID, err := apptrace.ParseID(spanIDStr)
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
		Trace      *apptrace.Trace
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

	return a.renderTemplate(w, r, "traces.html", http.StatusOK, &struct {
		TemplateCommon
		Traces []*apptrace.Trace
	}{
		Traces: traces,
	})
}
