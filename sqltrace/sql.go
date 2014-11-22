package sqltrace

import (
	"time"

	"sourcegraph.com/sourcegraph/apptrace"
)

type SQLEvent struct {
	SQL        string
	Tag        string
	ClientSend time.Time
	ClientRecv time.Time
}

func (SQLEvent) Schema() string { return "SQL" }

func (e SQLEvent) Start() time.Time { return e.ClientSend }
func (e SQLEvent) End() time.Time   { return e.ClientRecv }

func init() { apptrace.RegisterEvent(SQLEvent{}) }
