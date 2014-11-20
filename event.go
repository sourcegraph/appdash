package apptrace

import "time"

// An Event is a record of the occurrence of something.
type Event interface {
	Schema() string
}

// A spanName event sets a span's name.
type spanName string

func (spanName) Schema() string { return nameKey }

// Msg returns an Event that contains only a human-readable
// msg.
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
// contains only a human-readable msg.
func Log(msg string) Event {
	return logEvent{Msg: msg, Time: time.Now()}
}

type logEvent struct {
	Msg  string
	Time time.Time
}

func (logEvent) Schema() string { return "log" }

func (e *logEvent) Timestamp() time.Time { return e.Time }
