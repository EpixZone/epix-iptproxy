// Command epix-iptproxy builds a tiny C library (shared or archive) that exposes
// a 3-function Snowflake client API for EpixNet, wrapping tladesignz/IPtProxy's
// Controller. It runs the Snowflake pluggable transport in-process (no
// subprocess, so it works on iOS) and exposes a local SOCKS port that arti dials
// its bridge through.
//
// Build (per target, in CI):
//
//	c-shared -> .dll/.so   (Windows, Linux, Android; loaded at runtime)
//	c-archive -> .a        (macOS, iOS; linked statically)
//
// The exported C symbols are consumed by EpixNet's `iptproxy-sys` crate.
package main

// #include <stdlib.h>
import "C"

import (
	"os"
	"path/filepath"
	"sync"

	// IPtProxy's module path carries a `.git` suffix and its package is
	// `IPtProxy`, so it must be aliased.
	IPtProxy "github.com/tladesignz/IPtProxy.git"
)

var (
	mu   sync.Mutex
	ctrl *IPtProxy.Controller
)

// noopEvents satisfies IPtProxy.OnTransportEvents. The controller calls these on
// its own goroutine and would panic on a nil delegate, so we pass a no-op.
type noopEvents struct{}

func (noopEvents) Stopped(string, error) {}
func (noopEvents) Error(string, error)   {}
func (noopEvents) Connected(string)      {}

// EpixStartSnowflake starts the in-process Snowflake client. `stateDir` is where
// the transport keeps its state and log; the other arguments are the Snowflake
// rendezvous parameters (comma-separated where plural). Returns 0 on success, a
// negative code on failure. Idempotent: a second call while running is a no-op.
//
//export EpixStartSnowflake
func EpixStartSnowflake(stateDir, ice, broker, fronts, ampcache *C.char) C.int {
	mu.Lock()
	defer mu.Unlock()
	if ctrl != nil {
		return 0
	}
	dir := C.GoString(stateDir)
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "epix-snowflake")
	}
	c := IPtProxy.NewController(dir, false, false, "", noopEvents{})
	if c == nil {
		return -1
	}
	c.SnowflakeIceServers = C.GoString(ice)
	c.SnowflakeBrokerUrl = C.GoString(broker)
	c.SnowflakeFrontDomains = C.GoString(fronts)
	c.SnowflakeAmpCacheUrl = C.GoString(ampcache)
	if err := c.Start(IPtProxy.Snowflake, ""); err != nil {
		return -2
	}
	ctrl = c
	return 0
}

// EpixSnowflakePort returns the local SOCKS port the Snowflake client listens
// on, or 0 if it is not running.
//
//export EpixSnowflakePort
func EpixSnowflakePort() C.int {
	mu.Lock()
	defer mu.Unlock()
	if ctrl == nil {
		return 0
	}
	return C.int(ctrl.Port(IPtProxy.Snowflake))
}

// EpixStopSnowflake stops the Snowflake client and releases its listener.
//
//export EpixStopSnowflake
func EpixStopSnowflake() {
	mu.Lock()
	defer mu.Unlock()
	if ctrl == nil {
		return
	}
	ctrl.Stop(IPtProxy.Snowflake)
	ctrl = nil
}

func main() {}
