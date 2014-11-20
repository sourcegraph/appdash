package apptrace

type mockCollector struct {
	Collect_ func(SpanID, ...Annotation) error
}

func (c mockCollector) Collect(id SpanID, as ...Annotation) error {
	return c.Collect_(id, as...)
}
