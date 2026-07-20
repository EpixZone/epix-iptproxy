# epix-iptproxy

In-process Snowflake pluggable transport for [EpixNet](https://github.com/EpixZone/EpixNet).

On a network that blocks direct Tor, arti cannot reach any guard relay.
Snowflake gets through by rendezvousing with ephemeral volunteer WebRTC proxies.
This repo builds a tiny C library that runs the Snowflake **client** in-process
(no subprocess, so it works on iOS) and exposes a local SOCKS port. EpixNet's
arti client then dials its bridge through that port as an unmanaged pluggable
transport.

It is a thin cgo wrapper around the Tor Project's
[Snowflake client library](https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/snowflake)
(pure Go, no subprocess): it accepts SOCKS connections and pipes each through a
Snowflake transport connection. Exposes three C functions:

```c
int  EpixStartSnowflake(char *stateDir, char *ice, char *broker, char *fronts, char *ampcache);
int  EpixSnowflakePort(void);   // 0 until the listener binds
void EpixStopSnowflake(void);
```

## Why a separate repo

EpixNet is pure Rust and cross-compiles with rustls everywhere, so a Go
dependency does not belong in its build. This repo isolates the Go/cgo build:
CI compiles the wrapper for every target EpixNet ships and publishes the
artifacts as a release. EpixNet's `iptproxy-sys` crate downloads the pinned
release, exactly like the wallet is built in `epix-wallet` and downloaded by
`epix-browser`'s `build.rs`.

## Artifacts

On a version tag (`vX.Y.Z`), CI publishes one asset per Rust target triple:

| Target triple | Build mode | Asset | Linkage in EpixNet |
| --- | --- | --- | --- |
| `x86_64-pc-windows-msvc` | c-shared | `epix_snowflake-<triple>.dll` | loaded at runtime |
| `x86_64-unknown-linux-gnu` | c-archive | `…-<triple>.a` | static |
| `aarch64-linux-android` | c-shared | `…-<triple>.so` | loaded at runtime (jniLibs) |
| `aarch64-apple-darwin`, `x86_64-apple-darwin` | c-archive | `…-<triple>.a` | static |
| `aarch64-apple-ios`, `aarch64-apple-ios-sim` | c-archive | `…-<triple>.a` | static |

A shared build (`.dll`/`.so`) keeps the Go runtime and all its OS imports inside
the library, so a Windows MSVC binary consumes it with no MinGW/MSVC object
mixing and no extra link flags. iOS forbids loading user dylibs, so there it is
a static `.a`.

## Build locally

Needs Go 1.25. Desktop example:

```sh
go mod tidy
CGO_ENABLED=1 go build -buildmode=c-shared -o epix_snowflake.dll .   # or .so
```

Pins: Snowflake 2.14.1. Bump `go.mod` and re-tag to update.

## Consuming from EpixNet

`crates/iptproxy-sys/iptproxy.rev` pins the release tag of this repo. That
crate's `build.rs` downloads the asset for the build target; override with
`IPTPROXY_LIB_DIR=/path/to/dir` to use a local build.
