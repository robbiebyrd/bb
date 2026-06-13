//go:build !linux

package obd

import (
	"fmt"
	"time"
)

// openRFCOMM is unsupported off Linux. macOS and Windows expose a paired
// Bluetooth OBD adapter as a serial device (a /dev/cu.* path on macOS, a COMx
// port on Windows), so use that device path as the URI instead of an
// "rfcomm://<MAC>" URI — the result is the same Transport.
func openRFCOMM(target Target, _ time.Duration) (Transport, error) {
	return nil, fmt.Errorf(
		"native rfcomm:// connections are only supported on Linux; on this OS, pair %s and use its serial device path (macOS: /dev/cu.*, Windows: COMx) as INTERFACE_URI",
		target.MACString,
	)
}
