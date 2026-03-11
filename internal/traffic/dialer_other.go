//go:build !linux

package traffic

import (
	"net"
	"time"
)

// newBoundDialer returns a dialer bound to a local IP. SO_BINDTODEVICE is
// Linux-only, so on other platforms we fall back to IP-only binding.
func newBoundDialer(_ string, localIP string, timeout time.Duration) *net.Dialer {
	d := &net.Dialer{
		Timeout: timeout,
	}

	if localIP != "" {
		d.LocalAddr = &net.TCPAddr{IP: net.ParseIP(localIP)}
	}

	return d
}
