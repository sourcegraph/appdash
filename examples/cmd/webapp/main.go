package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"sourcegraph.com/sourcegraph/appdash"
	"sourcegraph.com/sourcegraph/appdash/httptrace"
	"sourcegraph.com/sourcegraph/appdash/traceapp"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/context"
	"github.com/gorilla/mux"
)

// Used to store the SpanID in a request's context (see gorilla/context docs
// for more information).
const CtxSpanID = 0

var collector appdash.Collector

func main() {
	// Create a recent in-memory store, evicting data after 20s.
	memStore := appdash.NewMemoryStore()
	store := &appdash.RecentStore{
		MinEvictAge: 20 * time.Second,
		DeleteStore: memStore,
	}

	// We can start the web UI on a separate port, as part of our app (another
	// alternative would be to connect to a centralized Appdash collection
	// server).
	tapp := traceapp.New(nil)
	tapp.Store = store
	tapp.Queryer = memStore
	log.Println("Appdash web UI running on HTTP :8700")
	go func() {
		log.Fatal(http.ListenAndServe(":8700", tapp))
	}()

	// We will use a local collector, as we are running the Appdash web UI
	// embedded within our app (see above).
	collector = appdash.NewLocalCollector(store)

	// Create the appdash/httptrace middleware.
	tracemw := httptrace.Middleware(collector, &httptrace.MiddlewareConfig{
		RouteName: func(r *http.Request) string { return r.URL.Path },
		SetContextSpan: func(r *http.Request, spanID appdash.SpanID) {
			context.Set(r, CtxSpanID, spanID)
		},
	})

	// Setup our router:
	router := mux.NewRouter()
	router.HandleFunc("/", Home)
	router.HandleFunc("/endpoint", Endpoint)

	// Setup Negroni for our app:
	n := negroni.Classic()
	n.Use(negroni.HandlerFunc(tracemw)) // Register appdash's HTTP middleware.
	n.UseHandler(router)
	n.Run(":8699")
}

// Home is the homepage handler for our app.
func Home(w http.ResponseWriter, r *http.Request) {
	// Grab the span from the gorilla context.
	span := context.Get(r, CtxSpanID).(appdash.SpanID)

	// We're going to make some API requests, so we create a HTTP client here.
	httpClient := &http.Client{
		Transport: &httptrace.Transport{
			Recorder: appdash.NewRecorder(span, collector),
			SetName:  true,
		},
	}

	// Make three API requests.
	for i := 0; i < 3; i++ {
		resp, err := httpClient.Get("http://localhost:8699/endpoint")
		if err != nil {
			log.Println("/endpoint:", err)
			continue
		}
		defer resp.Body.Close()
	}

	// Render the page.
	fmt.Fprintf(w, `<p>Three API requests have been made!</p>`)
	fmt.Fprintf(w, `<p><a href="http://localhost:8700/traces/%s" target="_">View the trace (ID:%s)</a></p>`, span.Trace, span.Trace)
}

// Endpoint is an example API endpoint. The backend of your service for
// example needs to contact several external or internal API endpoints.
func Endpoint(w http.ResponseWriter, r *http.Request) {
	time.Sleep(200 * time.Millisecond)
	fmt.Fprintf(w, "Slept for 200ms!")
}
