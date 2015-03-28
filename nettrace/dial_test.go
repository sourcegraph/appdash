package nettrace

import (
	"net"
	"testing"
	"time"

	"sourcegraph.com/sourcegraph/appdash"
)

func TestMakeTraceDialer(t *testing.T) {
	ms := appdash.NewMemoryStore()
	rec := appdash.NewRecorder(appdash.SpanID{1, 2, 3}, appdash.NewLocalCollector(ms))

	d := &mockDialer{}
	td := MakeTraceDialer(rec, d.Dial)

	c, _ := td("tcp", "localhost:1234")

	data := "Some data"
	c.Write([]byte(data))

	r := make([]byte, 32)
	c.Read(r)
	c.Close()

	ts, err := ms.Traces()
	if err != nil {
		t.Fatalf("Could not retrieve traces from memory store")
	}

	if len(ts) != 1 {
		t.Fatalf("Wrong span count found: ", len(ts))
	}
}

type mockDialer struct{}

func (d *mockDialer) Dial(network, address string) (net.Conn, error) {
	addr, _ := net.ResolveTCPAddr(network, address)
	return &mockConn{a: addr}, nil
}

type mockConn struct {
	a net.Addr
	d []byte
}

func (c *mockConn) Read(d []byte) (int, error) {
	copy(d, c.d)
	return len(d), nil
}

func (c *mockConn) Write(d []byte) (int, error) {
	c.d = d
	return len(c.d), nil
}

func (c *mockConn) Close() error {
	return nil
}

// LocalAddr implements the net.Conn interface.
func (c *mockConn) LocalAddr() net.Addr {
	return c.a
}

// RemoteAddr implements the net.Conn interface.
func (c *mockConn) RemoteAddr() net.Addr {
	return c.a
}

// SetDeadline implements the net.Conn interface.
func (c *mockConn) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline implements the net.Conn interface.
func (c *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline implements the net.Conn interface.
func (c *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}
