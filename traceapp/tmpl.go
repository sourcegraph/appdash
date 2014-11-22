package traceapp

import (
	"bytes"
	"fmt"
	"go/build"
	htmpl "html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"sourcegraph.com/sourcegraph/apptrace"

	"github.com/gorilla/mux"
)

var (
	// TemplateDir is the directory containing the html/template template files.
	TemplateDir = filepath.Join(defaultBase("sourcegraph.com/sourcegraph/apptrace/traceapp"), "tmpl")

	// ReloadTemplates is whether to reload html/template templates
	// before each request. It is useful during development.
	ReloadTemplates = true
)

var templates = [][]string{
	{"root.html", "layout.html"},
	{"trace.html", "trace.inc.html", "layout.html"},
	{"traces.html", "trace.inc.html", "layout.html"},
}

// TemplateCommon is data that is passed to (and available to) all templates.
type TemplateCommon struct {
	CurrentRoute string
	CurrentURI   *url.URL
	BaseURL      *url.URL
}

func (a *App) renderTemplate(w http.ResponseWriter, r *http.Request, name string, status int, data interface{}) error {
	a.tmplLock.Lock()
	defer a.tmplLock.Unlock()

	if a.tmpls == nil || ReloadTemplates {
		if err := a.parseHTMLTemplates(templates); err != nil {
			return err
		}
	}

	w.WriteHeader(status)
	if ct := w.Header().Get("content-type"); ct == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	t := a.tmpls[name]
	if t == nil {
		return fmt.Errorf("Template %s not found", name)
	}

	if data != nil {
		// Set TemplateCommon values.
		baseURL, err := a.URLTo(RootRoute)
		if err != nil {
			return err
		}
		reflect.ValueOf(data).Elem().FieldByName("TemplateCommon").Set(reflect.ValueOf(TemplateCommon{
			CurrentRoute: mux.CurrentRoute(r).GetName(),
			CurrentURI:   r.URL,
			BaseURL:      baseURL,
		}))
	}

	// Write to a buffer to properly catch errors and avoid partial output written to the http.ResponseWriter
	var buf bytes.Buffer
	err := t.Execute(&buf, data)
	if err != nil {
		return err
	}
	_, err = buf.WriteTo(w)
	return err
}

func (a *App) parseHTMLTemplates(sets [][]string) error {
	a.tmpls = map[string]*htmpl.Template{}
	for _, set := range sets {
		t := htmpl.New("")
		t.Funcs(htmpl.FuncMap{
			"urlTo":             a.URLTo,
			"urlToTrace":        a.URLToTrace,
			"itoa":              strconv.Itoa,
			"str":               func(v interface{}) string { return fmt.Sprintf("%s", v) },
			"durationClass":     durationClass,
			"filterAnnotations": filterAnnotations,
			"d3timeline":        d3timeline,
		})
		_, err := t.ParseFiles(joinTemplateDir(TemplateDir, set)...)
		if err != nil {
			return fmt.Errorf("template %v: %s", set, err)
		}
		t = t.Lookup("ROOT")
		if t == nil {
			return fmt.Errorf("ROOT template not found in %v", set)
		}
		a.tmpls[set[0]] = t
	}
	return nil
}

func joinTemplateDir(base string, files []string) []string {
	result := make([]string, len(files))
	for i := range files {
		result[i] = filepath.Join(base, files[i])
	}
	return result
}

func defaultBase(path string) string {
	p, err := build.Default.Import(path, "", build.FindOnly)
	if err != nil {
		log.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	p.Dir, err = filepath.Rel(cwd, p.Dir)
	if err != nil {
		log.Fatal(err)
	}
	return p.Dir
}

func durationClass(usec int64) string {
	msec := usec / 1000
	if msec < 30 {
		return "d0"
	} else if msec < 60 {
		return "d1"
	} else if msec < 90 {
		return "d2"
	} else if msec < 150 {
		return "d3"
	} else if msec < 250 {
		return "d4"
	} else if msec < 400 {
		return "d5"
	} else if msec < 600 {
		return "d6"
	} else if msec < 900 {
		return "d7"
	} else if msec < 1300 {
		return "d8"
	} else if msec < 1900 {
		return "d9"
	}
	return "d10"
}

func filterAnnotations(anns apptrace.Annotations) apptrace.Annotations {
	var anns2 apptrace.Annotations
	for _, ann := range anns {
		if ann.Key != "" && !strings.HasPrefix(ann.Key, "_") {
			anns2 = append(anns2, ann)
		}
	}
	return anns2

}
