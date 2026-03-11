//go:build linux

package traffic

import (
	"net"
	"syscall"
	"time"
)

// newBoundDialer returns a dialer that forces traffic through a specific
// network device using SO_BINDTODEVICE. This ensures packets egress through
// the correct physical interface regardless of the routing table.
func newBoundDialer(deviceName string, localIP string, timeout time.Duration) *net.Dialer {
	d := &net.Dialer{
		Timeout: timeout,
		Control: func(_, _ string, c syscall.RawConn) error {
			var soerr error
			if err := c.Control(func(fd uintptr) {
				soerr = syscall.SetsockoptString(
					int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, deviceName,
				)
			}); err != nil {
				return err
			}
			return soerr
		},
	}

	if localIP != "" {
		d.LocalAddr = &net.TCPAddr{IP: net.ParseIP(localIP)}
	}

	return d
}
