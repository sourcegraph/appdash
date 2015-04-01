package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"sourcegraph.com/sourcegraph/appdash"
	"sourcegraph.com/sourcegraph/appdash/sqltrace"
)

func init() {
	rand.Seed(time.Now().Unix())
}

func sampleData(c appdash.Collector) error {
	const (
		numTraces        = 60
		numSpansPerTrace = 7
	)
	log.Printf("Adding sample data (%d traces with %d spans each)", numTraces, numSpansPerTrace)
	for i := appdash.ID(1); i <= numTraces; i++ {
		traceID := appdash.NewRootSpanID()
		traceRec := appdash.NewRecorder(traceID, c)
		traceRec.Name(fakeHosts[rand.Intn(len(fakeHosts))])
		traceRec.Event(fakeEvent())

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
			rec.Event(fakeEvent())

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

// fakeEvent returns a SQLEvent with random send and recieve times.
func fakeEvent() *sqltrace.SQLEvent {
	send := time.Now().Add(-time.Duration(rand.Intn(30000)) * time.Millisecond)
	return &sqltrace.SQLEvent{
		ClientSend: send,
		ClientRecv: send.Add(time.Duration(rand.Intn(30000)) * time.Millisecond),
		SQL:        "SELECT * FROM table_name;",
		Tag:        fmt.Sprintf("fakeTag%d", rand.Intn(10)),
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

var fakeHosts = []string{
	"api.phafsea.org",
	"web.kraesey.net",
	"www3.bleland.com",
	"mun.moonuiburg",
	"zri.ozruamwe.ll",
	"e.rento",
	"go.na",
	"fre.nce",
	"hiu.wront",
	"shu.plin:9090",
	"luoron.net",
	"api.eproling.org",
	"iw.ruuh.ville",
	"riphero.ugh",
	"sek.hun.sea",
	"api.ye.ry",
	"fia.com",
	"jouver.io",
	"strayolis.io",
	"grisaso.io",
}
