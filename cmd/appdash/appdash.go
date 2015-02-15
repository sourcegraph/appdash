package main

import (
	"log"
	_ "net/http/pprof"
	"os"

	"github.com/jessevdk/go-flags"
)

// CLI is the go-flags CLI object that parses command-line arguments and runs commands.
var CLI = flags.NewNamedParser("appdash", flags.Default)

// GlobalOpt contains global options.
var GlobalOpt struct {
	Verbose bool `short:"v" description:"show verbose output"`
}

func init() {
	CLI.LongDescription = "appdash is an application tracing system"
	CLI.AddGroup("Global options", "", &GlobalOpt)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("")
	if _, err := CLI.Parse(); err != nil {
		os.Exit(1)
	}
}
