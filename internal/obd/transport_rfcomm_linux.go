//go:build linux

package obd

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Bluetooth socket protocols (not exported by x/sys/unix).
const (
	btprotoL2CAP  = 0
	btprotoRFCOMM = 3
	sdpPSM        = 0x0001 // L2CAP PSM of the SDP server
)

// defaultRFCOMMChannel is the SPP channel used when none is given and SDP
// discovery is unavailable or fails. Channel 1 is the de-facto SPP default and
// what the OBDLink MX+ uses.
const defaultRFCOMMChannel = 1

// rawSockaddrRC mirrors the kernel struct sockaddr_rc.
type rawSockaddrRC struct {
	family  uint16
	bdaddr  [6]byte
	channel uint8
	_       byte // pad to 10 bytes
}

// rawSockaddrL2 mirrors the kernel struct sockaddr_l2.
type rawSockaddrL2 struct {
	family     uint16
	psm        uint16 // little-endian
	bdaddr     [6]byte
	cid        uint16
	bdaddrType uint8
	_          byte // pad
}

// rfcommTransport is a native RFCOMM socket presented as a Transport. Reads use
// SO_RCVTIMEO so a read timeout surfaces as (0, nil) — matching the serial
// transport — letting the monitor loop poll its stop channel between reads.
type rfcommTransport struct {
	fd     int
	mac    string
	closed atomic.Bool
}

func (t *rfcommTransport) Read(p []byte) (int, error) {
	n, err := unix.Read(t.fd, p)
	if err != nil {
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK || err == unix.EINTR {
			return 0, nil // read timeout: no data, not an error
		}
		if t.closed.Load() {
			return 0, io.EOF
		}
		return 0, fmt.Errorf("rfcomm read from %s: %w", t.mac, err)
	}
	if n == 0 {
		return 0, io.EOF // peer closed the link
	}
	return n, nil
}

func (t *rfcommTransport) Write(p []byte) (int, error) {
	return unix.Write(t.fd, p)
}

func (t *rfcommTransport) Close() error {
	t.closed.Store(true)
	return unix.Close(t.fd)
}

// openRFCOMM connects to a Bluetooth device over RFCOMM. When no channel is
// configured it tries to discover the SPP channel over SDP, falling back to the
// default channel.
func openRFCOMM(target Target, readTimeout time.Duration) (Transport, error) {
	channel := target.Channel
	if channel == 0 {
		if ch, err := discoverSPPChannel(target.MAC, readTimeout); err == nil {
			channel = ch
		} else {
			channel = defaultRFCOMMChannel
		}
	}

	fd, err := unix.Socket(unix.AF_BLUETOOTH, unix.SOCK_STREAM, btprotoRFCOMM)
	if err != nil {
		return nil, fmt.Errorf("creating rfcomm socket: %w", err)
	}

	sa := rawSockaddrRC{family: unix.AF_BLUETOOTH, bdaddr: bdaddr(target.MAC), channel: channel}
	if _, _, errno := unix.Syscall(unix.SYS_CONNECT, uintptr(fd), uintptr(unsafe.Pointer(&sa)), unsafe.Sizeof(sa)); errno != 0 {
		_ = unix.Close(fd)
		return nil, rfcommConnectError(target.MACString, errno)
	}

	if readTimeout <= 0 {
		readTimeout = 50 * time.Millisecond
	}
	tv := unix.NsecToTimeval(readTimeout.Nanoseconds())
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("setting rfcomm read timeout: %w", err)
	}

	return &rfcommTransport{fd: fd, mac: target.MACString}, nil
}

// discoverSPPChannel queries the device's SDP server over L2CAP for the Serial
// Port Profile's RFCOMM channel.
func discoverSPPChannel(mac [6]byte, readTimeout time.Duration) (uint8, error) {
	fd, err := unix.Socket(unix.AF_BLUETOOTH, unix.SOCK_SEQPACKET, btprotoL2CAP)
	if err != nil {
		return 0, fmt.Errorf("creating l2cap socket: %w", err)
	}
	defer unix.Close(fd)

	sa := rawSockaddrL2{family: unix.AF_BLUETOOTH, psm: sdpPSM, bdaddr: bdaddr(mac)}
	if _, _, errno := unix.Syscall(unix.SYS_CONNECT, uintptr(fd), uintptr(unsafe.Pointer(&sa)), unsafe.Sizeof(sa)); errno != 0 {
		return 0, fmt.Errorf("connecting to sdp server: %w", errno)
	}

	if readTimeout <= 0 {
		readTimeout = time.Second
	}
	tv := unix.NsecToTimeval((2 * readTimeout).Nanoseconds())
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		return 0, err
	}

	if _, err := unix.Write(fd, buildServiceSearchAttributeRequest(1)); err != nil {
		return 0, fmt.Errorf("sending sdp request: %w", err)
	}

	buf := make([]byte, 4096)
	n, err := unix.Read(fd, buf)
	if err != nil {
		return 0, fmt.Errorf("reading sdp response: %w", err)
	}
	return parseRFCOMMChannel(buf[:n])
}

// rfcommConnectError maps a connect errno to an actionable message, surfacing the
// most common Bluetooth setup problem — the device not being paired/trusted.
func rfcommConnectError(mac string, errno unix.Errno) error {
	switch errno {
	case unix.EACCES, unix.EPERM:
		return fmt.Errorf("permission denied connecting to %s — pair and trust it first: `bluetoothctl pair %s` then `bluetoothctl trust %s`", mac, mac, mac)
	case unix.EHOSTDOWN, unix.ECONNREFUSED, unix.EHOSTUNREACH, unix.ENETDOWN:
		return fmt.Errorf("cannot reach %s — ensure it is powered, in range, and paired (`bluetoothctl pair %s`)", mac, mac)
	case unix.ETIMEDOUT:
		return fmt.Errorf("timed out connecting to %s — device out of range or not discoverable", mac)
	case unix.EBUSY:
		return fmt.Errorf("%s is busy — another process may hold the connection (e.g. a stale `rfcomm bind`)", mac)
	default:
		return fmt.Errorf("connecting to %s over rfcomm: %w", mac, errno)
	}
}
