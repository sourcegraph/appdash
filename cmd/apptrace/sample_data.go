package main

import (
	"fmt"
	"log"

	"sourcegraph.com/sourcegraph/apptrace"
)

func sampleData(c apptrace.Collector) error {
	const (
		numTraces        = 5
		numSpansPerTrace = 7
	)
	log.Printf("Adding sample data (%d traces with %d spans each)", numTraces, numSpansPerTrace)
	for i := apptrace.ID(1); i <= numTraces; i++ {
		traceID := apptrace.NewRootSpanID()
		var lastSpanID apptrace.SpanID
		for j := apptrace.ID(0); j < numSpansPerTrace; j++ {
			var spanID apptrace.SpanID
			if j == 0 {
				spanID = traceID // root
			} else if j == 1 {
				spanID = apptrace.NewSpanID(traceID) // parent is root
			} else {
				spanID = apptrace.NewSpanID(lastSpanID) // parent is predecessor
			}

			rec := apptrace.NewRecorder(spanID, c)
			rec.Name(fakeNames[int(j+i)%len(fakeNames)])
			if j%3 == 0 {
				rec.Log("hello")
			}
			if j%5 == 0 {
				rec.Msg("hi")
			}

			if errs := rec.Errors(); len(errs) > 0 {
				return fmt.Errorf("recorder errors: %v", errs)
			}

			lastSpanID = spanID
		}
	}
	return nil
}

var fakeNames = []string{
	"Phafsea",
	"Kraesey",
	"Bleland",
	"Moonuiburg",
	"Zriozruamwell",
	"Erento",
	"Gona",
	"Frence",
	"Hiuwront",
	"Shuplin",
	"Luoron",
	"Eproling",
	"Iwruuhville",
	"Ripherough",
	"Sekhunsea",
	"Yery",
	"Fia",
	"Jouver",
	"Strayolis",
	"Grisaso",
}
