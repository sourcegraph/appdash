// +build danger

// Run with:
//
//  go test -tags=danger -memprofilerate=1 -memprofile=mem.out -bench=BenchmarkChunkedCollector1mil -run=NONE -v
//
// (danger tag is because it is very slow and eats a lot of memory!)

package appdash

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

// BenchmarkChunkedCollector1mil performs 1 million collects with the same set
// of annotations and prints several runtime memory statistics during the
// process, invoking runtime.GC and runtime/debug.FreeOSMemory ten times as the
// final step.
func BenchmarkChunkedCollector1mil(b *testing.B) {
	cc := &ChunkedCollector{
		Collector: collectorFunc(func(span SpanID, anns ...Annotation) error {
			return nil
		}),
		MinInterval: time.Millisecond * 1,
	}
	const (
		nCollections = 1000000
		nAnnotations = 50
	)
	anns := make([]Annotation, nAnnotations)
	for i := range anns {
		anns[i] = Annotation{Key: "k", Value: []byte{'v'}}
	}

	memStats := &memStats{
		Collections: nCollections,
	}

	memStats.Log("pre")
	var x ID
	for i := 0; i < b.N; i++ {
		for c := 0; c < nCollections; c++ {
			x++
			err := cc.Collect(SpanID{x, x + 1, x + 2}, anns...)
			if err != nil {
				b.Fatal(err)
			}
		}
	}

	memStats.Log("post")

	// Perform N garbage collections (multiple are needed because the GC can at
	// times need two phases to collect everything, we use an overly large
	// amount just to be sure).
	fmt.Println("[garbage collections]")
	nGC := 10
	for i := 0; i < nGC; i++ {
		start := time.Now()
		runtime.GC()
		fmt.Printf("  %v. GC - %v\n", i, time.Since(start))
	}
	fmt.Println("")
	memStats.Log("after GC")

	// Perform N debug.FreeOSMemory() calls, again performing multiple just to
	// besure.
	fmt.Println("[free os memory]")
	nFree := 10
	for i := 0; i < nFree; i++ {
		start := time.Now()
		debug.FreeOSMemory()
		fmt.Printf("  %v. FreeOSMemory - %v\n", i, time.Since(start))
	}
	fmt.Println("")
	memStats.Log("after GC & FreeOSMemory")

	fmt.Printf("sleeping 30s; check memory usage with 'top -p %v' now", os.Getpid())
	time.Sleep(30 * time.Second)
}
