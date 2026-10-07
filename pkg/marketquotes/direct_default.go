//go:build !android || !cgo

package marketquotes

import (
	"context"
	"net"
	"time"
)

// Proxy=nil skips HTTP proxies, not TUN routes. Desktop full-tunnel policies
// still require the VPN's split-routing rules; this function cannot override OS policy.
func platformDirectDial(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, network, address)
}
