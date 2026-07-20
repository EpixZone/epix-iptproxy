// Command epix-iptproxy builds a tiny C library (shared or archive) that exposes
// a 3-function Snowflake client API for EpixNet. It runs the Snowflake
// pluggable transport in-process (no subprocess, so it works on iOS) and serves
// a local SOCKS5 listener that arti dials its bridge through as an unmanaged
// pluggable transport.
//
// Build (per target, in CI):
//
//	c-shared -> .dll/.so   (Windows, Linux, Android; loaded at runtime)
//	c-archive -> .a        (macOS, iOS; linked statically)
//
// It wraps the Tor Project's Snowflake client library directly (pure Go, no
// subprocess), doing what a SOCKS-facing Snowflake client does: accept SOCKS
// connections and pipe each through a Snowflake transport connection.
package main

// #include <stdlib.h>
import "C"

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync"

	pt "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/goptlib"
	sf "gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake/v2/client/lib"
)

var (
	mu       sync.Mutex
	listener *pt.SocksListener
)

// EpixStartSnowflake starts the in-process Snowflake client and its local SOCKS
// listener. `stateDir` is accepted for API stability but unused (the client
// library keeps no on-disk state). The other arguments are the Snowflake
// rendezvous parameters (comma-separated where plural). Returns 0 on success, a
// negative code on failure. Idempotent while running.
//
//export EpixStartSnowflake
func EpixStartSnowflake(stateDir, ice, broker, fronts, ampcache *C.char) C.int {
	_ = stateDir
	mu.Lock()
	defer mu.Unlock()
	if listener != nil {
		return 0
	}
	transport, err := sf.NewSnowflakeClient(sf.ClientConfig{
		BrokerURL:    C.GoString(broker),
		AmpCacheURL:  C.GoString(ampcache),
		FrontDomains: splitComma(C.GoString(fronts)),
		ICEAddresses: splitComma(C.GoString(ice)),
		Max:          1,
	})
	if err != nil {
		return -1
	}
	ln, err := pt.ListenSocks("tcp", "127.0.0.1:0")
	if err != nil {
		return -2
	}
	listener = ln
	go acceptLoop(ln, transport)
	return 0
}

// EpixSnowflakePort returns the local SOCKS port, or 0 if not running.
//
//export EpixSnowflakePort
func EpixSnowflakePort() C.int {
	mu.Lock()
	defer mu.Unlock()
	if listener == nil {
		return 0
	}
	if a, ok := listener.Addr().(*net.TCPAddr); ok {
		return C.int(a.Port)
	}
	return 0
}

// EpixStopSnowflake stops the SOCKS listener.
//
//export EpixStopSnowflake
func EpixStopSnowflake() {
	mu.Lock()
	defer mu.Unlock()
	if listener != nil {
		_ = listener.Close()
		listener = nil
	}
}

func acceptLoop(ln *pt.SocksListener, transport *sf.Transport) {
	for {
		conn, err := ln.AcceptSocks()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue // transient SOCKS handshake error; keep serving
		}
		go handle(conn, transport)
	}
}

func handle(conn *pt.SocksConn, transport *sf.Transport) {
	defer conn.Close()
	remote, err := transport.Dial()
	if err != nil {
		_ = conn.Reject()
		return
	}
	defer remote.Close()
	if err := conn.Grant(&net.TCPAddr{IP: net.IPv4zero, Port: 0}); err != nil {
		return
	}
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(remote, conn); done <- struct{}{} }()
	go func() { _, _ = io.Copy(conn, remote); done <- struct{}{} }()
	<-done
}

func splitComma(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func main() {}
