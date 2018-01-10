package appdash

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"sort"

	"sourcegraph.com/sourcegraph/appdash/internal/wire"
)

func TestCollectorServer(t *testing.T) {
	var (
		packets   []*wire.CollectPacket
		packetsMu sync.Mutex
	)
	mc := collectorFunc(func(span SpanID, anns ...Annotation) error {
		packetsMu.Lock()
		defer packetsMu.Unlock()
		packets = append(packets, newCollectPacket(span, anns))
		return nil
	})

	l, err := net.Listen("tcp4", ":0")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("collector listening on %s", l.Addr())

	cs := NewServer(l, mc)
	go cs.Start()

	cc := &collectorT{t, NewRemoteCollector(l.Addr().String())}

	collectPackets := []*wire.CollectPacket{
		newCollectPacket(SpanID{1, 2, 3}, Annotations{{"k1", []byte("v1")}}),
		newCollectPacket(SpanID{2, 3, 4}, Annotations{{"k2", []byte("v2")}}),
	}
	for _, p := range collectPackets {
		cc.MustCollect(spanIDFromWire(p.Spanid), annotationsFromWire(p.Annotation)...)
	}
	if err := cc.Collector.(*RemoteCollector).Close(); err != nil {
		t.Error(err)
	}

	time.Sleep(20 * time.Millisecond)

	packetsMu.Lock()
	defer packetsMu.Unlock()
	if !reflect.DeepEqual(packets, collectPackets) {
		t.Errorf("server collected %v, want %v", packets, collectPackets)
	}
}

func TestCollectorServer_stress(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	const (
		n             = 1000
		allowFailures = 0
		errorEvery    = 1000000
	)

	var (
		packets   = map[SpanID]struct{}{}
		packetsMu sync.RWMutex
	)
	mc := collectorFunc(func(span SpanID, anns ...Annotation) error {
		packetsMu.Lock()
		defer packetsMu.Unlock()
		packets[span] = struct{}{}
		// log.Printf("Added %v", span)

		// Occasional errors, which should cause the client to
		// reconnect.
		if len(packets)%(errorEvery+1) == 0 {
			return errors.New("fake error")
		}
		return nil
	})

	l, err := net.Listen("tcp4", ":0")
	if err != nil {
		t.Fatal(err)
	}

	cs := NewServer(l, mc)
	go cs.Start()

	cc := &collectorT{t, NewRemoteCollector(l.Addr().String())}
	// cc.Collector.(*RemoteCollector).Debug = true

	want := make(map[SpanID]struct{}, n)
	for i := 0; i < n; i++ {
		id := NewRootSpanID()
		want[id] = struct{}{}
		cc.MustCollect(id)
	}
	if err := cc.Collector.(*RemoteCollector).Close(); err != nil {
		t.Error(err)
	}

	time.Sleep(20 * time.Millisecond)
	var missing []string
	packetsMu.RLock()
	for spanID := range want {
		if _, present := packets[spanID]; !present {
			missing = append(missing, fmt.Sprintf("span %v was not collected", spanID))
		}
	}
	packetsMu.RUnlock()
	if len(missing) > allowFailures {
		for _, missing := range missing {
			t.Error(missing)
		}
	}
}

func BenchmarkRemoteCollector1000(b *testing.B) {
	const (
		nCollections = 1000
		nAnnotations = 100
	)

	l, err := net.Listen("tcp", ":0")
	if err != nil {
		b.Fatal(err)
	}

	cs := NewServer(l, collectorFunc(func(span SpanID, anns ...Annotation) error {
		return nil
	}))
	go cs.Start()

	c := NewRemoteCollector(l.Addr().String())
	// cc.Collector.(*RemoteCollector).Debug = true
	for n := 0; n < b.N; n++ {
		for nc := 0; nc < nCollections; nc++ {
			id := NewRootSpanID()
			anns := make([]Annotation, nAnnotations)
			for a := range anns {
				anns[a] = Annotation{"k1", []byte("v1")}
			}
			if err := c.Collect(NewRootSpanID(), anns...); err != nil {
				b.Fatalf("Collect(%+v, %v): %s", id, anns, err)
			}
		}
	}

	if err := c.Close(); err != nil {
		b.Error(err)
	}
}

func TestTLSCollectorServer(t *testing.T) {
	var (
		numPackets   int
		numPacketsMu sync.RWMutex
	)
	mc := collectorFunc(func(span SpanID, anns ...Annotation) error {
		numPacketsMu.Lock()
		defer numPacketsMu.Unlock()
		numPackets++
		return nil
	})

	l, err := tls.Listen("tcp", "127.0.0.1:0", &localhostTLSConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("TLS collector listening on %s", l.Addr())

	cs := NewServer(l, mc)
	go cs.Start()

	cc := &collectorT{t, NewTLSRemoteCollector(l.Addr().String(), &localhostTLSConfig)}
	cc.MustCollect(SpanID{1, 2, 3})
	cc.MustCollect(SpanID{2, 3, 4})
	if err := cc.Collector.(*RemoteCollector).Close(); err != nil {
		t.Error(err)
	}

	time.Sleep(20 * time.Millisecond)
	numPacketsMu.RLock()
	defer numPacketsMu.RUnlock()
	if want := 2; numPackets != want {
		t.Errorf("server collected %d packets, want %d", numPackets, want)
	}
}

func TestChunkedCollector(t *testing.T) {
	var (
		packets   []*wire.CollectPacket
		packetsMu sync.RWMutex
	)

	mc := collectorFunc(func(span SpanID, anns ...Annotation) error {
		packetsMu.Lock()
		defer packetsMu.Unlock()
		packets = append(packets, newCollectPacket(span, anns))
		return nil
	})

	cc := &ChunkedCollector{
		Collector:   mc,
		MinInterval: time.Millisecond * 10,
	}
	cc.Collect(SpanID{1, 2, 3}, Annotation{"k1", []byte("v1")})
	cc.Collect(SpanID{1, 2, 3}, Annotation{"k2", []byte("v2")})
	cc.Collect(SpanID{2, 3, 4}, Annotation{"k3", []byte("v3")})
	cc.Collect(SpanID{1, 2, 3}, Annotation{"k4", []byte("v4")})

	// Check before the MinInterval has elapsed.
	packetsMu.RLock()
	if len(packets) != 0 {
		t.Errorf("before MinInterval: got len(packets) == %d, want 0", len(packets))
	}
	packetsMu.RUnlock()

	time.Sleep(cc.MinInterval * 2)

	// Check after the MinInterval has elapsed.
	want := []*wire.CollectPacket{
		newCollectPacket(SpanID{1, 2, 3}, Annotations{{"k1", []byte("v1")}, {"k2", []byte("v2")}, {"k4", []byte("v4")}}),
		newCollectPacket(SpanID{2, 3, 4}, Annotations{{"k3", []byte("v3")}}),
	}
	sort.Sort(byTraceID(want))
	packetsMu.Lock()
	sort.Sort(byTraceID(packets))
	if !reflect.DeepEqual(packets, want) {
		t.Errorf("after MinInterval: got packets == %v, want %v", packets, want)
	}
	lenBeforeStop := len(packets)
	packetsMu.Unlock()

	// Check that Stop stops it.
	cc.Stop()
	cc.Collect(SpanID{1, 2, 3}, Annotation{"k5", []byte("v5")})
	time.Sleep(cc.MinInterval * 2)
	packetsMu.RLock()
	if len(packets) != lenBeforeStop {
		t.Errorf("after Stop: got len(packets) == %d, want %d", len(packets), lenBeforeStop)
	}
	packetsMu.RUnlock()
}

func TestChunkedCollectorFlushTimeout(t *testing.T) {
	mc := collectorFunc(func(span SpanID, anns ...Annotation) error {
		time.Sleep(200 * time.Millisecond) // Slow collector
		return nil
	})

	cc := &ChunkedCollector{
		Collector:    mc,
		MinInterval:  10 * time.Millisecond,
		FlushTimeout: 1 * time.Second,
	}

	for i := 0; i < 100; i++ {
		cc.Collect(NewRootSpanID(), Annotation{"k1", []byte("v1")})
	}

	err := cc.Flush()
	if err != ErrQueueDropped {
		t.Fatal("got", err, "expected", ErrQueueDropped)
	}
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if len(cc.pendingBySpanID) != 0 {
		t.Fatal("got", len(cc.pendingBySpanID), "queued but expected 0")
	}
}

// collectorFunc implements the Collector interface by calling the function.
type collectorFunc func(SpanID, ...Annotation) error

// Collect implements the Collector interface by calling the function itself.
func (c collectorFunc) Collect(id SpanID, as ...Annotation) error {
	return c(id, as...)
}

type collectorT struct {
	t *testing.T
	Collector
}

func (s collectorT) MustCollect(id SpanID, as ...Annotation) {
	if err := s.Collector.Collect(id, as...); err != nil {
		s.t.Fatalf("Collect(%+v, %v): %s", id, as, err)
	}
}

var localhostTLSConfig tls.Config

func init() {
	cert, err := tls.X509KeyPair(localhostCert, localhostKey)
	if err != nil {
		panic(fmt.Sprintf("localhostTLSConfig: %v", err))
	}
	localhostTLSConfig.Certificates = []tls.Certificate{cert}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(localhostCert); !ok {
		panic("AppendCertsFromPEM: !ok")
	}
	localhostTLSConfig.RootCAs = certPool

	localhostTLSConfig.BuildNameToCertificate()
}

// localhostCert is a PEM-encoded TLS cert with SAN IPs
// "127.0.0.1" and "[::1]", expiring at the last second of 2049 (the end
// of ASN.1 time).
// generated from src/crypto/tls:
// go run generate_cert.go  --rsa-bits 512 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIBjzCCATmgAwIBAgIRAKOMbj1tSId/UIw8iy+WOsYwDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAgFw03MDAxMDEwMDAwMDBaGA8yMDg0MDEyOTE2
MDAwMFowEjEQMA4GA1UEChMHQWNtZSBDbzBcMA0GCSqGSIb3DQEBAQUAA0sAMEgC
QQDSCuKkkCiQucKc3JxehtEJ2R2kf2HyN0Nv+WM9b3V/k+XP0bM8YdH5mCL3tv+D
dRhDZweBGjaCfjftrkSRchpjAgMBAAGjaDBmMA4GA1UdDwEB/wQEAwICpDATBgNV
HSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MC4GA1UdEQQnMCWCC2V4
YW1wbGUuY29thwR/AAABhxAAAAAAAAAAAAAAAAAAAAABMA0GCSqGSIb3DQEBCwUA
A0EAdBLKWCH2P8vLBeOMRN49+YdkFZbpuMZ/VeRqba6WSjOhRrMAZMKbhjuJLRi4
1jP+GHPZBroLQXlPtAsroVE1fg==
-----END CERTIFICATE-----`)

// localhostKey is the private key for localhostCert.
var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBOgIBAAJBANIK4qSQKJC5wpzcnF6G0QnZHaR/YfI3Q2/5Yz1vdX+T5c/Rszxh
0fmYIve2/4N1GENnB4EaNoJ+N+2uRJFyGmMCAwEAAQJALMKhFcyauGy9qkvhDsvQ
FDcud/WlW8anGl+c5GSyN2NcL1omkOQHmkVqAx1xLbId+1KzH8YOR7TyDDy2DFzW
0QIhAPL/GA2qbLovJLstNm0cJiEt+q4YwmTB6tQz2hYGXMZnAiEA3UhXlN7cMQH5
BxIu9deviOSBdR09pI3jdeNHJM7/tqUCIFKhqH5NK/gMPANilpV38wdpaUt2o/Q7
dS2ADHNc6oOVAiEAuX7dPESd3M9EjHLnvtpxoZW8GArNE9aFqNs/VlHX9qkCIFI8
nwoPqw0BHjIJFwQDxA7UZAx75riOvYxv1jvgc3XR
-----END RSA PRIVATE KEY-----`)

func BenchmarkChunkedCollector500(b *testing.B) {
	cc := &ChunkedCollector{
		Collector: collectorFunc(func(span SpanID, anns ...Annotation) error {
			return nil
		}),
		MinInterval: time.Millisecond * 10,
	}
	const (
		nCollections = 500
		nAnnotations = 50
	)
	var x ID
	for i := 0; i < b.N; i++ {
		for c := 0; c < nCollections; c++ {
			anns := make([]Annotation, nAnnotations)
			for i := range anns {
				anns[i] = Annotation{Key: "k", Value: []byte{'v'}}
			}
			x++
			err := cc.Collect(SpanID{x, x + 1, x + 2}, anns...)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

type byTraceID []*wire.CollectPacket

func (bt byTraceID) Len() int           { return len(bt) }
func (bt byTraceID) Swap(i, j int)      { bt[i], bt[j] = bt[j], bt[i] }
func (bt byTraceID) Less(i, j int) bool { return *bt[i].Spanid.Trace < *bt[j].Spanid.Trace }
