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
// go run generate_cert.go  --rsa-bits 1024 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIICFDCCAX2gAwIBAgIRAM1e/rerac2oQxwFRBs6xqUwDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAgFw03MDAxMDEwMDAwMDBaGA8yMDg0MDEyOTE2
MDAwMFowEjEQMA4GA1UEChMHQWNtZSBDbzCBnzANBgkqhkiG9w0BAQEFAAOBjQAw
gYkCgYEArjblqiSC1oT9oc9jvIh+piH8qQCThdqvszzWJmWPqlsout2GlcjCn2SA
3K1G/8oZ/J8LIKbQGsNp21n0DkVfGcwyekUSsDZrEPxulCNLx13yAB2/8XvX5Emj
PHLu1aeulGBkSnEGTHiawU0mdsM3p673yBig9y8R+HvPtI+T8bECAwEAAaNoMGYw
DgYDVR0PAQH/BAQDAgKkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1UdEwEB/wQF
MAMBAf8wLgYDVR0RBCcwJYILZXhhbXBsZS5jb22HBH8AAAGHEAAAAAAAAAAAAAAA
AAAAAAEwDQYJKoZIhvcNAQELBQADgYEAgNTf1Is6y+GJ0Rue/EeP1Ma7WijLIng9
8uuq1WbHxVqjrIKWozuiwdwxw7yJ/ZNFZQWnIhNsvvCv5o+/OitE9aBD0aIlL19H
r5GL0AUV1WZ2pOm9Jt4vPD2yIscdHlhpKEk9hmMjZ7+NJbSh9olqL/0wP4N53au3
mk5EtX6hoaU=
-----END CERTIFICATE-----`)

// localhostKey is the private key for localhostCert.
var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIICdgIBADANBgkqhkiG9w0BAQEFAASCAmAwggJcAgEAAoGBAK425aokgtaE/aHP
Y7yIfqYh/KkAk4Xar7M81iZlj6pbKLrdhpXIwp9kgNytRv/KGfyfCyCm0BrDadtZ
9A5FXxnMMnpFErA2axD8bpQjS8dd8gAdv/F71+RJozxy7tWnrpRgZEpxBkx4msFN
JnbDN6eu98gYoPcvEfh7z7SPk/GxAgMBAAECgYAYGjo+Bt0fJrkcaN/olo3HGE6n
ZxAB5daHGrSaDVUKAaCp8boMAQGEIdh+L27yNpjPzYUxmEKUYVLE6TYNv2U/pvv9
SU08f0NMymIsaXmsioq/tjlVNm4omVSZ3ejBaKSYo8PHMkVki0CeU4eUl99WpUCE
5JW/VJadpCJGzepbDQJBANpGEeWKwnLxxDkxtZkuIq0LloSit/p5zG1f2GH4HqxW
g2f+wXzj3ek8QswIqWpuAG4T7CyCnOCJg7i4rxaIXncCQQDMU1eYA1Fx03qC1iFv
tt7DRbljjXGHXve3TIiAlZuyPtEfWRztkUjn0oyBHnnuF6ZjyHCEK5GbsuK91za1
PHMXAkBa1J3N75hLTOBjDJSNUe2MJS5Vs4Dr8pNnUGMzIZViEf5M4G6UEh7eV/1T
+qbFa1EyfYfiXdf6eD8gN3pk3gqxAkAjQ7MXgmMZISXA1RI6RLaXvz3q56uTcJmS
Yjwg7TFNBzhyj5/FhNCvahBj7I2gwSYvjJWWyio8VBh8KVvA1ekLAkEAulkqgAu8
yzC70s8yDM++lkQgqdURzkaQaS49FHGDfdFzy+9M5M8nN87pV3TgHbHs4yPmk9Un
LJa2t1+nE3B3UA==
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
