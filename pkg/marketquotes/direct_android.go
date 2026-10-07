//go:build android && cgo

package marketquotes

/*
#cgo LDFLAGS: -landroid
#include <android/multinetwork.h>
#include <errno.h>
static int cy_bind_network(uint64_t handle, int fd) {
 if (android_setsocknetwork((net_handle_t)handle, fd) == 0) return 0;
 return errno;
}
*/
import "C"

import (
	"context"
	"net"
	"syscall"
	"time"
)

func platformDirectDial(ctx context.Context, network, address string) (net.Conn, error) {
	d := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
	state := platformNetwork.Load()
	if state == nil || state.handle == 0 || len(state.dns) == 0 {
		return d.DialContext(ctx, network, address)
	}
	control := func(_, _ string, raw syscall.RawConn) error {
		var bindErr error
		err := raw.Control(func(fd uintptr) {
			if code := C.cy_bind_network(C.uint64_t(state.handle), C.int(fd)); code != 0 {
				bindErr = syscall.Errno(code)
			}
		})
		if err != nil {
			return err
		}
		return bindErr
	}
	d.Control = control
	// Resolve on the same physical network: a VPN fake-IP answer cannot be used
	// on a socket bound to Wi-Fi. Never redirect overseas provider DNS this way.
	d.Resolver = &net.Resolver{PreferGo: true, Dial: func(c context.Context, n, _ string) (net.Conn, error) {
		var last error
		for _, server := range state.dns {
			attempt, cancel := context.WithTimeout(c, time.Second)
			conn, err := (&net.Dialer{Timeout: time.Second, Control: control}).DialContext(attempt, n, net.JoinHostPort(server, "53"))
			cancel()
			if err == nil {
				return conn, nil
			}
			last = err
		}
		return nil, last
	}}
	conn, err := d.DialContext(ctx, network, address)
	if err == nil || ctx.Err() != nil {
		return conn, err
	}
	// Some VPNs disallow explicit underlying sockets (including lockdown). Respect
	// that decision and fall back to the system route, rather than changing policy.
	return (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, network, address)
}
