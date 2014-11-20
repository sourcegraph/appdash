package apptrace

// A Collector collects events that occur in spans.
type Collector interface {
	Collect(SpanID, ...Annotation) error
}
