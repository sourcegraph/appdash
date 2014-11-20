package apptrace

// A Recorder is associated with a span and records annotations on the
// span.
type Recorder interface {
	Record(Event)
}
