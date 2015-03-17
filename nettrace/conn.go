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

// traceConn is a net.Conn implementation that emits send and /recv
// events
type traceConn struct {
	base net.Conn
	rec  *appdash.Recorder
}

// Read implements io.Reade
func (c traceConn) Read(b []byte) (n int, err error) {
	ev := NewConnReadEvent()
	ev.ReadStart = time.Now()
	n, err = c.base.Read(b)
	ev.ReadEnd = time.Now()
	ev.ReadCount = n
	if err != nil {
		ev.Error = err.Error()
	}

	c.rec.Event(ev)

	return n, err
}

// Write implements io.Writer
func (c traceConn) Write(b []byte) (n int, err error) {
	ev := NewConnWriteEvent()
	ev.WriteStart = time.Now()
	n, err = c.base.Write(b)
	ev.WriteEnd = time.Now()
	ev.WriteCount = n
	if err != nil {
		ev.Error = err.Error()
	}

	c.rec.Event(ev)

	return n, err
}

// Close implements the Closer interface
func (c traceConn) Close() error {
	return c.base.Close()
}

// LocalAddr implements net.Conn's LocalAddr
func (c traceConn) LocalAddr() net.Addr {
	return c.base.LocalAddr()
}

// RemoteAddr implements net.Conn's RemoteAddr
func (c traceConn) RemoteAddr() net.Addr {
	return c.base.RemoteAddr()
}

// SetDeadline implements net.Conn's SetDeadline
func (c traceConn) SetDeadline(t time.Time) error {
	return c.base.SetDeadline(t)
}

// SetReadDeadline implements net.Conn's SetReadDeadline
func (c traceConn) SetReadDeadline(t time.Time) error {
	return c.base.SetReadDeadline(t)
}

// SetWriteDeadline implements net.Conn's SetWriteDeadline
func (c traceConn) SetWriteDeadline(t time.Time) error {
	return c.base.SetWriteDeadline(t)
}

// NewConnReadEvent creates a new conection write event, noting the
// volume and time taken reading data on a connection
func NewConnReadEvent() *ConnReadEvent {
	return &ConnReadEvent{}
}

// ConnEvent records an connection event.
type ConnReadEvent struct {
	ReadCount int
	Error     string
	ReadStart time.Time
	ReadEnd   time.Time
}

// Schema returns the constant "HTTPClient".
func (ConnReadEvent) Schema() string { return "ConnRead" }

// Important implements the appdash ImportantEvent.
func (ConnReadEvent) Important() []string {
	return []string{"ReadStart", "ReadEnd", "ReadCount"}
}

// Start implements the appdash TimespanEvent interface.
func (e ConnReadEvent) Start() time.Time { return e.ReadStart }

// End implements the appdash TimespanEvent interface.
func (e ConnReadEvent) End() time.Time { return e.ReadEnd }

// NewConnWriteEvent creates a new conection write event, noting the
// volume and time taken to write data on a connection
func NewConnWriteEvent() *ConnWriteEvent {
	return &ConnWriteEvent{}
}

// ConnEvent records an connection event.
type ConnWriteEvent struct {
	WriteCount int
	Error      string
	WriteStart time.Time
	WriteEnd   time.Time
}

// Schema returns the constant "HTTPClient".
func (ConnWriteEvent) Schema() string { return "ConnWrite" }

// Important implements the appdash ImportantEvent.
func (ConnWriteEvent) Important() []string {
	return []string{"WriteStart", "WriteEnd", "WriteCount"}
}

// Start implements the appdash TimespanEvent interface.
func (e ConnWriteEvent) Start() time.Time { return e.WriteStart }

// End implements the appdash TimespanEvent interface.
func (e ConnWriteEvent) End() time.Time { return e.WriteEnd }
