package main

import (
	"log"
	"net/http"

	"sourcegraph.com/sourcegraph/appdash"
	"sourcegraph.com/sourcegraph/appdash/examples/cmd/webapp"
	"sourcegraph.com/sourcegraph/appdash/httptrace"
	"sourcegraph.com/sourcegraph/appdash/traceapp"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	influxdb "github.com/influxdb/influxdb/cmd/influxd/run"
)

const CtxSpanID = 0

var collector appdash.Collector

func main() {
	conf, err := influxdb.NewDemoConfig()
	if err != nil {
		log.Fatalf("failed to create influxdb config, error: %v", err)
	}
	store, err := appdash.NewInfluxDBStore(conf, &influxdb.BuildInfo{})
	if err != nil {
		log.Fatalf("failed to create influxdb store, error: %v", err)
	}
	tapp := traceapp.New(nil)
	tapp.Store = store
	tapp.Queryer = store
	log.Println("Appdash web UI running on HTTP :8700")
	go func() {
		log.Fatal(http.ListenAndServe(":8700", tapp))
	}()
	collector = appdash.NewLocalCollector(store)
	tracemw := httptrace.Middleware(collector, &httptrace.MiddlewareConfig{
		RouteName: func(r *http.Request) string { return r.URL.Path },
		SetContextSpan: func(r *http.Request, spanID appdash.SpanID) {
			context.Set(r, CtxSpanID, spanID)
		},
	})
	router := mux.NewRouter()
	router.HandleFunc("/", webapp.Home)
	router.HandleFunc("/endpoint", webapp.Endpoint)
	n := negroni.Classic()
	n.Use(negroni.HandlerFunc(tracemw)) // Register appdash's HTTP middleware.
	n.UseHandler(router)
	n.Run(":8699")
}
