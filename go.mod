module github.com/EpixZone/epix-iptproxy

go 1.25.0

// IPtProxy carries v5 tags but its module path has no /v5, so Go addresses it
// with the +incompatible suffix. `go mod tidy` (run in CI) writes go.sum.
require github.com/tladesignz/IPtProxy.git v5.5.1+incompatible
