package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"

	"sourcegraph.com/sourcegraph/apptrace"
	"sourcegraph.com/sourcegraph/apptrace/traceapp"
)

func init() {
	_, err := CLI.AddCommand("serve",
		"start an apptrace server",
		"The serve command starts an apptrace server.",
		&serveCmd,
	)
	if err != nil {
		log.Fatal(err)
	}
}

type ServeCmd struct {
	CollectorAddr string `long:"collector" description:"collector listen address" default:":7701"`
	HTTPAddr      string `long:"http" description:"HTTP listen address" default:":7700"`
	SampleData    bool   `long:"sample-data" description:"add sample data"`

	Debug bool `short:"d" long:"debug" description:"debug log"`
	Trace bool `long:"trace" description:"trace log"`

	TLSCert string `long:"tls-cert" description:"TLS certificate file (if set, enables TLS)"`
	TLSKey  string `long:"tls-key" description:"TLS key file (if set, enables TLS)"`
}

var serveCmd ServeCmd

func (c *ServeCmd) Execute(args []string) error {
	var (
		Store   = apptrace.NewMemoryStore()
		Queryer = Store
	)

	app := traceapp.New(nil)
	app.Store = Store
	app.Queryer = Queryer

	if c.SampleData {
		sampleData(Store)
	}

	var l net.Listener
	var proto string
	if c.TLSCert != "" || c.TLSKey != "" {
		certBytes, err := ioutil.ReadFile(c.TLSCert)
		if err != nil {
			log.Fatal(err)
		}
		keyBytes, err := ioutil.ReadFile(c.TLSKey)
		if err != nil {
			log.Fatal(err)
		}

		var tc tls.Config
		cert, err := tls.X509KeyPair(certBytes, keyBytes)
		if err != nil {
			log.Fatal(err)
		}
		tc.Certificates = []tls.Certificate{cert}
		l, err = tls.Listen("tcp", c.CollectorAddr, &tc)
		if err != nil {
			log.Fatal(err)
		}
		proto = fmt.Sprintf("TLS cert %s, key %s", c.TLSCert, c.TLSKey)
	} else {
		var err error
		l, err = net.Listen("tcp", c.CollectorAddr)
		if err != nil {
			log.Fatal(err)
		}
		proto = "plaintext TCP (no security)"
	}
	log.Printf("apptrace collector listening on %s (%s)", c.CollectorAddr, proto)
	cs := apptrace.NewServer(l, apptrace.NewLocalCollector(Store))
	cs.Debug = c.Debug
	cs.Trace = c.Trace
	go cs.Start()

	if c.TLSCert != "" || c.TLSKey != "" {
		log.Printf("apptrace HTTPS server listening on %s (TLS cert %s, key %s)", c.HTTPAddr, c.TLSCert, c.TLSKey)
		return http.ListenAndServeTLS(c.HTTPAddr, c.TLSCert, c.TLSKey, app)
	} else {
		log.Printf("apptrace HTTP server listening on %s", c.HTTPAddr)
		return http.ListenAndServe(c.HTTPAddr, app)
	}
}
