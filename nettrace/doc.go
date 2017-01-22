// Package nettrace implement a net.Dial and net.Conn that trace connections
// reads, ands writes. An example of hooking this into the http Transport is below
//
//		defaultDial := (&net.Dialer{
//			Timeout:   30 * time.Second,
//			KeepAlive: 30 * time.Second,
//		}).Dial
//		traceDial := MakeTraceDialer(rec, defaultDial)
//
//		// A customized version of http.DefaultTransport
//		netTraceTransport := &http.Transport{
//			Proxy:               http.ProxyFromEnvironment,
//			Dial:                traceDial,
//			TLSHandshakeTimeout: 10 * time.Second,
//		}
//
//		httpClient := &http.Client{
//			Transport: &httptrace.Transport{Recorder: rec, SetName: true, Transport: netTraceTransport},
//		}
//
// This will cause connections, read and writes made by the httpClient to generate events.
package nettrace
