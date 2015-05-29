package appdash

// Verify the event type satisfies the interfaces.
var _ = EventMarshaler(AggregateEvent{})
var _ = EventUnmarshaler(AggregateEvent{})
var _ = Event(AggregateEvent{})

// TODO(slimsag): write regression tests for AggregateStore
