package nettrace

import (
	"net"
	"time"

	"sync"

	"sourcegraph.com/sourcegraph/appdash"
)

// spanMap maintains a  map of connection we have seen, and the
// recorder being used to track events on that connection
var spanMap *connSpanMap

func init() {
	spanMap = &connSpanMap{
		lock: &sync.RWMutex{},
		smap: make(map[net.Conn]*appdash.Recorder),
	}
	appdash.RegisterEvent(ConnEvent{})
}

// MakeTraceDialer returns a Dial function that wraps the passed Dialer
// and records connection timing information to the provided appdash
// Reocrder
func MakeTraceDialer(r *appdash.Recorder, defaultDial func(network string, address string) (net.Conn, error)) func(network string, address string) (net.Conn, error) {
	return func(network string, address string) (net.Conn, error) {
		begin := time.Now()
		conn, err := defaultDial(network, address)
		conned := time.Now()

		cr, ok := spanMap.get(conn)
		if !ok {
			cr = r.Child()
			cr.Name("net.Conn")
			spanMap.set(conn, cr)

			ce := NewConnEvent(conn)
			ce.Opened = begin
			ce.Connected = conned
			cr.Event(ce)
		}

		// TODO(tcm) This span should really be rooted off of the connection
		tconn := traceConn{
			base: conn,
			rec:  cr,
		}
		return tconn, err
	}
}

// NewConnEvent returns a new connetion event recording local, remote addresses
// along with connetion open and creation time.
func NewConnEvent(c net.Conn) *ConnEvent {
	return &ConnEvent{Connection: connInfo(c)}
}

// ConnInfo describes a net connection.
type ConnInfo struct {
	RemoteAddr string
	LocalAddr  string
}

func connInfo(c net.Conn) ConnInfo {
	return ConnInfo{
		RemoteAddr: c.RemoteAddr().String(),
		LocalAddr:  c.LocalAddr().String(),
	}
}

// connSpanMap thread safe map for tracking connections and spans
type connSpanMap struct {
	lock *sync.RWMutex
	smap map[net.Conn]*appdash.Recorder
}

// get the Recorder associated withe this connection
func (m *connSpanMap) get(c net.Conn) (*appdash.Recorder, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	r, ok := m.smap[c]
	return r, ok
}

// set the Recorder associated withe this connection
func (m *connSpanMap) set(c net.Conn, r *appdash.Recorder) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.smap[c] = r
	return
}

// ConnEvent records an connection event.
type ConnEvent struct {
	Connection ConnInfo
	Opened     time.Time
	Connected  time.Time
}

// Schema returns the constant "ConnOpen".
func (ConnEvent) Schema() string { return "ConnOpen" }

// Important implements the appdash ImportantEvent.
func (ConnEvent) Important() []string {
	return []string{"Opened", "Connected"}
}

// Start implements the appdash TimespanEvent interface.
func (e ConnEvent) Start() time.Time { return e.Opened }

// End implements the appdash TimespanEvent interface.
func (e ConnEvent) End() time.Time { return e.Connected }
