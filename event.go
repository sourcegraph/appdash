package apptrace

import (
	"fmt"
	"reflect"
	"time"
)

// An Event is a record of the occurrence of something.
type Event interface {
	Schema() string
}

// EventMarshaler is the interface implemented by an event that can
// marshal a representation of itself into annotations.
//
// TODO(sqs): implement this in MarshalEvent
type EventMarshaler interface {
	MarshalEvent() ([]*Annotation, error)
}

// EventUnmarshaler is the interface implemented by an event that can
// unmarshal an annotation representation of itself.
//
// TODO(sqs): implement this in UnmarshalEvent
type EventUnmarshaler interface {
	UnmarshalEvent([]*Annotation) error
}

const schemaPrefix = "_schema:"

// MarshalEvent marshals an event into annotations.
func MarshalEvent(e Event) (Annotations, error) {
	var as Annotations
	flattenValue("", reflect.ValueOf(e), func(k, v string) {
		as = append(as, Annotation{Key: k, Value: []byte(v)})
	})
	as = append(as, Annotation{Key: schemaPrefix + e.Schema()})
	return as, nil
}

// An EventSchemaUnmarshalError is when annotations are attempted to
// be unmarshaled into an event object that does not match any of the
// schemas in the annotations.
type EventSchemaUnmarshalError struct {
	Found  []string // schemas found in the annotations
	Target string   // schema of the target event
}

func (e *EventSchemaUnmarshalError) Error() string {
	return fmt.Sprintf("event: can't unmarshal annotations with schemas %v into event of schema %s", e.Found, e.Target)
}

// UnmarshalEvent unmarshals annotations into an event.
func UnmarshalEvent(as Annotations, e Event) error {
	aSchemas := as.schemas()
	schemaOK := false
	for _, s := range aSchemas {
		if s == e.Schema() {
			schemaOK = true
			break
		}
	}
	if !schemaOK {
		return &EventSchemaUnmarshalError{Found: aSchemas, Target: e.Schema()}
	}

	unflattenValue("", reflect.ValueOf(&e), reflect.TypeOf(&e), mapToKVs(as.StringMap()))
	return nil
}

// A spanName event sets a span's name.
type spanName string

func (spanName) Schema() string { return nameKey }

// Msg returns an Event that contains only a human-readable message.
func Msg(msg string) Event {
	return msgEvent{Msg: msg}
}

type msgEvent struct {
	Msg string
}

func (msgEvent) Schema() string { return "msg" }

// A TimestampedEvent is an Event with a timestamp.
type TimestampedEvent interface {
	Timestamp() time.Time
}

// Log returns an Event whose timestamp is the current time that
// contains only a human-readable message.
func Log(msg string) Event {
	return logEvent{Msg: msg, Time: time.Now()}
}

type logEvent struct {
	Msg  string
	Time time.Time
}

func (logEvent) Schema() string { return "log" }

func (e *logEvent) Timestamp() time.Time { return e.Time }
