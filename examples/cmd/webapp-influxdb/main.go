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
	"github.com/influxdata/influxdb/toml"
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

	// Enables retention policies which will be executed within an interval of 30 minutes.
	conf.Retention.Enabled = true
	conf.Retention.CheckInterval = toml.Duration(30 * time.Minute)

	// If you do not want metrics to be reported (see: https://docs.influxdata.com/influxdb/v0.10/administration/config/#reporting-disabled-false) uncomment the following line:
	//conf.ReportingDisabled = true

	// InfluxDB server auth credentials. If user does not exist yet it will
	// be created as admin user.
	user := appdash.InfluxDBAdminUser{Username: "demo", Password: "demo"}

	// Retention policy named "one_day_only" with a duration of "1d" - meaning db data older than "1d" will be deleted
	// with an interval checking set by `conf.Retention.CheckInterval`.
	// Minimum duration time is 1 hour ("1h") - See: github.com/influxdata/influxdb/issues/5198
	defaultRP := appdash.InfluxDBRetentionPolicy{Name: "one_day_only", Duration: "1d"}

	// Configure InfluxDB ports, if you desire:
	//conf.Admin.BindAddress = ":8083"
	//conf.BindAddress = ":8088"
	//conf.CollectdInputs[0].BindAddress = "" // auto-chosen
	//conf.GraphiteInputs[0].BindAddress = ":2003"
	//conf.HTTPD.BindAddress = ":8086"
	//conf.OpenTSDBInputs[0].BindAddress = ":4242"
	//conf.UDPInputs[0].BindAddress = "" // auto-chosen

	store, err := appdash.NewInfluxDBStore(appdash.InfluxDBStoreConfig{
		AdminUser: user,
		BuildInfo: &influxDBServer.BuildInfo{},
		DefaultRP: defaultRP,
		Server:    conf,
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
