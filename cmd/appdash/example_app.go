package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"sourcegraph.com/sourcegraph/apptrace"
	"sourcegraph.com/sourcegraph/apptrace/httptrace"
	"sourcegraph.com/sourcegraph/apptrace/traceapp"
)

func init() {
	_, err := CLI.AddCommand("demo",
		"start a demo web app that uses apptrace",
		"The demo command starts a demo web app that uses apptrace.",
		&demoCmd,
	)
	if err != nil {
		log.Fatal(err)
	}
}

type DemoCmd struct {
	ApptraceHTTPAddr string `long:"apptrace-http" description:"apptrace HTTP listen address" default:":8700"`
	DemoHTTPAddr     string `long:"demo-http" description:"demo app HTTP listen address" default:":8699"`
	Debug            bool   `long:"debug" description:"debug logging"`
	Trace            bool   `long:"trace" description:"trace logging"`
}

var demoCmd DemoCmd

func (c *DemoCmd) Execute(args []string) error {
	store := apptrace.NewMemoryStore()

	l, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		log.Fatal(err)
	}
	collectorPort := l.Addr().(*net.TCPAddr).Port
	log.Printf("Apptrace collector listening on tcp:%d", collectorPort)
	cs := apptrace.NewServer(l, apptrace.NewLocalCollector(store))
	cs.Debug = c.Debug
	cs.Trace = c.Trace
	go cs.Start()

	apptraceURLStr := "http://localhost" + c.ApptraceHTTPAddr
	apptraceURL, err := url.Parse(apptraceURLStr)
	if err != nil {
		log.Fatalf("Error parsing http://localhost:%s: %s", c.ApptraceHTTPAddr, err)
	}
	log.Printf("Apptrace web UI running at %s", apptraceURL)
	tapp := traceapp.New(nil)
	tapp.Store = store
	tapp.Queryer = store
	go func() {
		log.Fatal(http.ListenAndServe(c.ApptraceHTTPAddr, tapp))
	}()

	demoURLStr := "http://localhost" + c.DemoHTTPAddr
	demoURL, err := url.Parse(demoURLStr)
	if err != nil {
		log.Fatalf("Error parsing http://localhost:%s: %s", c.DemoHTTPAddr, err)
	}
	localCollector := apptrace.NewRemoteCollector(fmt.Sprintf(":%d", collectorPort))
	http.Handle("/", &middlewareHandler{
		middleware: httptrace.Middleware(localCollector, &httptrace.MiddlewareConfig{
			RouteName:      func(r *http.Request) string { return r.URL.Path },
			SetContextSpan: requestSpans.setRequestSpan,
		}),
		next: &demoApp{collector: localCollector, baseURL: demoURL, apptraceURL: apptraceURL},
	})
	log.Println()
	log.Printf("Apptrace demo app running at %s", demoURL)
	return http.ListenAndServe(c.DemoHTTPAddr, nil)
}

type middlewareHandler struct {
	middleware func(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc)
	next       http.Handler
}

func (h *middlewareHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.middleware(w, r, h.next.ServeHTTP)
}

type demoApp struct {
	collector   apptrace.Collector
	baseURL     *url.URL
	apptraceURL *url.URL
}

func (a *demoApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	span := requestSpans[r]

	switch r.URL.Path {
	case "/":
		io.WriteString(w, `<h1>Apptrace demo</h1>
<p>Welcome! Click some links and then view the traces for each HTTP request by following the link at the bottom of the page.
<ul>
<li><a href="/api-calls">Visit a page that issues some API calls</a></li>
</ul>`)
	case "/api-calls":
		httpClient := &http.Client{
			Transport: &httptrace.Transport{Recorder: apptrace.NewRecorder(span, a.collector), SetName: true},
		}
		resp, err := httpClient.Get(a.baseURL.ResolveReference(&url.URL{Path: "/endpoint-A"}).String())
		if err == nil {
			defer resp.Body.Close()
		}
		resp, err = httpClient.Get(a.baseURL.ResolveReference(&url.URL{Path: "/endpoint-B"}).String())
		if err == nil {
			defer resp.Body.Close()
		}
		resp, err = httpClient.Get(a.baseURL.ResolveReference(&url.URL{Path: "/endpoint-C"}).String())
		if err == nil {
			defer resp.Body.Close()
		}
		io.WriteString(w, `<a href="/">Home</a><br><br><p>I just made 3 API calls. Check the trace below to see them!</p>`)
	case "/endpoint-A":
		time.Sleep(250 * time.Millisecond)
		io.WriteString(w, "performed an operation!")
		return
	case "/endpoint-B":
		time.Sleep(75 * time.Millisecond)
		io.WriteString(w, "performed another operation!")
		return
	case "/endpoint-C":
		time.Sleep(300 * time.Millisecond)
		io.WriteString(w, "performed yet another operation!")
		return
	}

	spanURL := a.apptraceURL.ResolveReference(&url.URL{Path: fmt.Sprintf("/traces/%v", span.Trace)})
	io.WriteString(w, fmt.Sprintf(`<br><br><hr><a href="%s">View request trace on apptrace</a> (trace ID is %s)`, spanURL, span.Trace))
}

type requestSpanMap map[*http.Request]apptrace.SpanID

// requestSpans stores the apptrace span ID associated with each HTTP
// request. In a real app, you would use something like
// gorilla/context instead of a map (so that entries get removed when
// handling is completed, etc.).
var requestSpans = requestSpanMap{}

func (m requestSpanMap) setRequestSpan(r *http.Request, spanID apptrace.SpanID) {
	m[r] = spanID
}
