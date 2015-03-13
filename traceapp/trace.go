package traceapp

import "sourcegraph.com/sourcegraph/appdash"

// collectTrace asks the given collector to collect all of the spans and
// annotations in the given trace recursively. Any errors that occur during
// collectino are directly returned, breaking the recursive chain.
func collectTrace(c appdash.Collector, t *appdash.Trace) error {
	// Record this span's ID and annotations.
	err := c.Collect(t.ID, t.Annotations...)
	if err != nil {
		return err
	}

	// Descend into sub-spans.
	for _, sub := range t.Sub {
		err = collectTrace(c, sub)
		if err != nil {
			return err
		}
	}
	return nil
}
