package apptrace

// A Collector collects events that occur in spans.
type Collector interface {
	Collect(SpanID, ...Annotation) error
}

// NewLocalCollector returns a Collector that writes directly to a
// Store.
func NewLocalCollector(s Store) Collector {
	return s
}
