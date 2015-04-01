package traceapp

import "sourcegraph.com/sourcegraph/appdash"

// aggItem represents a set of traces with the name (label) and their cumulative
// time.
type aggItem struct {
	Label string `json:"label"`
	Value int64  `json:"value"`
}

// aggregate aggregates and encodes the given traces as JSON to the given writer.
func (a *App) aggregate(traces []*appdash.Trace) ([]*aggItem, error) {
	aggregated := make(map[string]*aggItem)
	for _, trace := range traces {
		// Calculate the cumulative time -- which we can already get through the
		// profile view's calculation method.
		_, prof, err := a.calcProfile(nil, trace)
		if err != nil {
			return nil, err
		}

		// Grab the aggregation item for the named trace, or create a new one if it
		// does not already exist.
		i, ok := aggregated[prof.Name]
		if !ok {
			i = &aggItem{Label: prof.Name}
			aggregated[prof.Name] = i
		}

		// Perform aggregation.
		i.Value += prof.TimeCum
		if i.Value == 0 {
			i.Value = 1 // Must be positive values or else d3pie won't render.
		}
	}

	// Form an array (d3pie needs a JSON array, not a map).
	list := make([]*aggItem, 0, len(aggregated))
	for _, item := range aggregated {
		list = append(list, item)
	}
	return list, nil
}
