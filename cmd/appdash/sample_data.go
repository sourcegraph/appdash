package main

import (
	"fmt"
	"log"
	"time"

	"sourcegraph.com/sourcegraph/appdash"
	"sourcegraph.com/sourcegraph/appdash/sqltrace"
)

func sampleData(c appdash.Collector) error {
	const (
		numTraces        = 5
		numSpansPerTrace = 7
	)
	log.Printf("Adding sample data (%d traces with %d spans each)", numTraces, numSpansPerTrace)
	for i := appdash.ID(1); i <= numTraces; i++ {
		traceID := appdash.NewRootSpanID()
		traceRec := appdash.NewRecorder(traceID, c)
		traceRec.Name("Request")
		traceRec.Event(fakeEvent("root", 0))

		lastSpanID := traceID
		for j := appdash.ID(1); j < numSpansPerTrace; j++ {
			// The parent span is the predecessor.
			spanID := appdash.NewSpanID(lastSpanID)

			rec := appdash.NewRecorder(spanID, c)
			rec.Name(fakeNames[int(j+i)%len(fakeNames)])
			if j%3 == 0 {
				rec.Log("hello")
			}
			if j%5 == 0 {
				rec.Msg("hi")
			}

			// Create a fake SQL event, subtracting one so we start at zero.
			rec.Event(fakeEvent("children", int(j-1)))

			// Check for any recorder errors.
			if errs := rec.Errors(); len(errs) > 0 {
				return fmt.Errorf("recorder errors: %v", errs)
			}

			lastSpanID = spanID
		}
	}
	return nil
}

var initTime = time.Now()

// fakeEvent returns a SQLEvent with fake send and recieve times from the
// fakeTimes map.
func fakeEvent(name string, i int) *sqltrace.SQLEvent {
	// Mod by the length of the slice to avoid index out of bounds when
	// sampleData.numSpansPerTrace > len(t).
	t := fakeTimes[name]
	i = i % len(t)
	return &sqltrace.SQLEvent{
		ClientSend: initTime.Add(t[i][0] * time.Millisecond),
		ClientRecv: initTime.Add(t[i][1] * time.Millisecond),
		SQL:        "SELECT * FROM table_name;",
		Tag:        fmt.Sprintf("fakeTag%d", i),
	}
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

// fakeTimes is a map of start and end times in milliseconds.
var fakeTimes = map[string][][2]time.Duration{
	"root": [][2]time.Duration{{5, 998}},
	"children": [][2]time.Duration{
		{11, 90},
		{92, 150},
		{154, 459},
		{462, 730},
		{734, 826},
		{823, 975},
		{983, 995},
	},
}
