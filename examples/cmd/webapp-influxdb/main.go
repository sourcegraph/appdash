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

	influxDBServer "github.com/influxdata/influxdb/cmd/influxd/run"
)

const CtxSpanID = 0

var collector appdash.Collector

func main() {
	conf, err := influxDBServer.NewDemoConfig()
	if err != nil {
		log.Fatalf("failed to create influxdb config, error: %v", err)
	}

	// Enables InfluxDB server authentication.
	conf.HTTPD.AuthEnabled = true

	// InfluxDB server auth credentials. If user does not exist yet it will
	// be created as admin user.
	user := appdash.InfluxDBAdminUser{Username: "demo", Password: "demo"}

	store, err := appdash.NewInfluxDBStore(appdash.InfluxDBStoreConfig{
		AdminUser: user,
		Server:    conf,
		BuildInfo: &influxDBServer.BuildInfo{},
	})
	if err != nil {
		log.Fatalf("failed to create influxdb store, error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Fatal(err)
		}
	}()
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
	router.HandleFunc("/", Home)
	router.HandleFunc("/endpoint", Endpoint)
	n := negroni.Classic()
	n.Use(negroni.HandlerFunc(tracemw))
	n.UseHandler(router)
	n.Run(":8699")
}

func Home(w http.ResponseWriter, r *http.Request) {
	span := context.Get(r, CtxSpanID).(appdash.SpanID)
	httpClient := &http.Client{
		Transport: &httptrace.Transport{
			Recorder: appdash.NewRecorder(span, collector),
			SetName:  true,
		},
	}
	for i := 0; i < 3; i++ {
		resp, err := httpClient.Get("http://localhost:8699/endpoint")
		if err != nil {
			log.Println("/endpoint:", err)
			continue
		}
		resp.Body.Close()
	}
	fmt.Fprintf(w, `<p>Three API requests have been made!</p>`)
	fmt.Fprintf(w, `<p><a href="http://localhost:8700/traces/%s" target="_">View the trace (ID:%s)</a></p>`, span.Trace, span.Trace)
}

func Endpoint(w http.ResponseWriter, r *http.Request) {
	time.Sleep(200 * time.Millisecond)
	fmt.Fprintf(w, "Slept for 200ms!")
}
