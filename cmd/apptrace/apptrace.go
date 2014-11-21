package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"sourcegraph.com/sourcegraph/apptrace"

	"github.com/jessevdk/go-flags"
)

// CLI is the go-flags CLI object that parses command-line arguments and runs commands.
var CLI = flags.NewNamedParser("apptrace", flags.Default)

// GlobalOpt contains global options.
var GlobalOpt struct {
	Verbose bool `short:"v" description:"show verbose output"`
}

func init() {
	CLI.LongDescription = "apptrace is an application tracing system"
	CLI.AddGroup("Global options", "", &GlobalOpt)
}

var (
	Store   = apptrace.NewMemoryStore()
	Queryer = Store
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("")
	if _, err := CLI.Parse(); err != nil {
		os.Exit(1)
	}
}

func init() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
}
