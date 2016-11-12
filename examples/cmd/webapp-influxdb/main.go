package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"sourcegraph.com/sourcegraph/appdash"
	"sourcegraph.com/sourcegraph/appdash/httptrace"
	"sourcegraph.com/sourcegraph/appdash/traceapp"
	"sourcegraph.com/sourcegraph/appdash/x/influxdbstore"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
)

type ctxSpanKey struct{}

var collector appdash.Collector

func main() {
	// Create a default InfluxDB configuration.
	conf, err := influxdbstore.NewConfig()
	if err != nil {
		log.Fatalf("failed to create influxdb config, error: %v", err)
	}

	// Enable InfluxDB server HTTP basic auth.
	conf.Server.HTTPD.AuthEnabled = true
	conf.AdminUser = influxdbstore.AdminUser{
		Username: "demo",
		Password: "demo",
	}

	// If you do not want metrics to be reported (see: https://docs.influxdata.com/influxdb/v0.10/administration/config/#reporting-disabled-false) uncomment the following line:
	//conf.Server.ReportingDisabled = true

	// Configure InfluxDB ports, if you desire:
	//conf.Server.Admin.BindAddress = ":8083"
	//conf.Server.BindAddress = ":8088"
	//conf.Server.CollectdInputs[0].BindAddress = "" // auto-chosen
	//conf.Server.GraphiteInputs[0].BindAddress = ":2003"
	//conf.Server.HTTPD.BindAddress = ":8086"
	//conf.Server.OpenTSDBInputs[0].BindAddress = ":4242"
	//conf.Server.UDPInputs[0].BindAddress = "" // auto-chosen

	// Control where InfluxDB server logs are written to, if desired:
	//conf.LogOutput = ioutil.Discard

	store, err := influxdbstore.New(conf)
	if err != nil {
		log.Fatalf("failed to create influxdb store, error: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Fatal(err)
		}
	}()
	url, err := url.Parse("http://localhost:8700")
	if err != nil {
		log.Fatal(err)
	}
	tapp, err := traceapp.New(nil, url)
	if err != nil {
		log.Fatal(err)
	}
	tapp.Store = store
	tapp.Queryer = store
	tapp.Aggregator = store
	log.Println("Appdash web UI running on HTTP :8700")
	go func() {
		log.Fatal(http.ListenAndServe(":8700", tapp))
	}()
	collector = appdash.NewLocalCollector(store)
	tracemw := httptrace.Middleware(collector, &httptrace.MiddlewareConfig{
		RouteName: func(r *http.Request) string { return r.URL.Path },
		SetContextSpan: func(r *http.Request, spanID appdash.SpanID) *http.Request {
			return r.WithContext(context.WithValue(r.Context(), ctxSpanKey{}, spanID))
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
	span := r.Context().Value(ctxSpanKey{}).(appdash.SpanID)
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
