package main

import (
	"log"
	"net/http"

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
	BindAddr   string `long:"http" description:"HTTP listen address" default:":7700"`
	SampleData bool   `long:"sample-data" description:"add sample data"`
}

var serveCmd ServeCmd

func (c *ServeCmd) Execute(args []string) error {
	app := traceapp.New(nil)
	app.Store = Store
	app.Queryer = Queryer

	if c.SampleData {
		sampleData(Store)
	}

	log.Printf("apptrace listening on %s", c.BindAddr)
	return http.ListenAndServe(c.BindAddr, app)
}
