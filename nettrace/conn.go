package nettrace

import (
	"net"
	"time"

	"sourcegraph.com/sourcegraph/appdash"
)

func init() {
	appdash.RegisterEvent(ConnReadEvent{})
	appdash.RegisterEvent(ConnWriteEvent{})
}

// traceConn is a net.Conn implementation that emits send and recv
// events.
type traceConn struct {
	base net.Conn
	rec  *appdash.Recorder
}

// Read implements io.Reader interface.
func (c traceConn) Read(b []byte) (n int, err error) {
	ev := ConnReadEvent{}
	ev.ReadStart = time.Now()
	n, err = c.base.Read(b)
	ev.ReadEnd = time.Now()
	ev.BytesRead = n
	if err != nil {
		ev.Error = err.Error()
	}

	c.rec.Event(ev)

	return n, err
}

// Write implements the io.Writer interface.
func (c traceConn) Write(b []byte) (n int, err error) {
	ev := ConnWriteEvent{}
	ev.WriteStart = time.Now()
	n, err = c.base.Write(b)
	ev.WriteEnd = time.Now()
	ev.BytesWritten = n
	if err != nil {
		ev.Error = err.Error()
	}

	c.rec.Event(ev)

	return n, err
}

// Close implements the io.Closer interface.
func (c traceConn) Close() error {
	return c.base.Close()
}

// LocalAddr implements the net.Conn interface.
func (c traceConn) LocalAddr() net.Addr {
	return c.base.LocalAddr()
}

// RemoteAddr implements the net.Conn interface.
func (c traceConn) RemoteAddr() net.Addr {
	return c.base.RemoteAddr()
}

// SetDeadline implements the net.Conn interface.
func (c traceConn) SetDeadline(t time.Time) error {
	return c.base.SetDeadline(t)
}

// SetReadDeadline implements the net.Conn interface.
func (c traceConn) SetReadDeadline(t time.Time) error {
	return c.base.SetReadDeadline(t)
}

// SetWriteDeadline implements the net.Conn interface.
func (c traceConn) SetWriteDeadline(t time.Time) error {
	return c.base.SetWriteDeadline(t)
}

// ConnEvent records a connection read event.
type ConnReadEvent struct {
	BytesRead int
	Error     string
	ReadStart time.Time
	ReadEnd   time.Time
}

// Schema returns the constant "ConnRead".
func (ConnReadEvent) Schema() string { return "ConnRead" }

// Important implements the appdash appdash.ImportantEvent interface.
func (ConnReadEvent) Important() []string {
	return []string{"ReadStart", "ReadEnd", "ReadCount"}
}

// Start implements the appdash.TimespanEvent interface.
func (e ConnReadEvent) Start() time.Time { return e.ReadStart }

// End implements the appdash.TimespanEvent interface.
func (e ConnReadEvent) End() time.Time { return e.ReadEnd }

// ConnWriteEvent records an connection write event.
type ConnWriteEvent struct {
	BytesWritten int
	Error        string
	WriteStart   time.Time
	WriteEnd     time.Time
}

// Schema returns the constant "ConnWrite".
func (ConnWriteEvent) Schema() string { return "ConnWrite" }

// Important implements the appdash.ImportantEvent interface.
func (ConnWriteEvent) Important() []string {
	return []string{"WriteStart", "WriteEnd", "WriteCount"}
}

// Start implements the appdash.TimespanEvent interface.
func (e ConnWriteEvent) Start() time.Time { return e.WriteStart }

// End implements the appdash.TimespanEvent interface.
func (e ConnWriteEvent) End() time.Time { return e.WriteEnd }
