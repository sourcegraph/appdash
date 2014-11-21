package traceapp

import (
	htmpl "html/template"
	"net/http"
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
	id, err := apptrace.ParseID(mux.Vars(r)["Trace"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	trace, err := a.Store.Trace(id)
	if err != nil {
		if err == apptrace.ErrTraceNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return nil
		}
		return err
	}

	return a.renderTemplate(w, r, "trace.html", http.StatusOK, &struct {
		TemplateCommon
		TraceID apptrace.ID
		Trace   *apptrace.Trace
	}{
		TraceID: id,
		Trace:   trace,
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
