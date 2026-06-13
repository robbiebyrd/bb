package obd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// TargetKind identifies how a connection URI should be opened.
type TargetKind int

const (
	// TargetSerial is a serial/USB or bound-rfcomm device path.
	TargetSerial TargetKind = iota
	// TargetRFCOMM is a Bluetooth device addressed by MAC, opened with a native
	// RFCOMM socket (no manual `rfcomm bind`).
	TargetRFCOMM
)

// Target is a parsed connection URI.
type Target struct {
	Kind TargetKind
	// Path is the serial device path (TargetSerial).
	Path string
	// MAC is the Bluetooth address bytes in network order, AA:BB:CC:DD:EE:FF =
	// {0xAA,0xBB,0xCC,0xDD,0xEE,0xFF} (TargetRFCOMM).
	MAC [6]byte
	// MACString is the human-readable MAC (TargetRFCOMM).
	MACString string
	// Channel is the RFCOMM channel; 0 means "discover via SDP, else default".
	Channel uint8
}

// macPattern matches a colon- or hyphen-separated 6-octet MAC address.
var macPattern = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}$`)

// ParseURI classifies a connection URI as a serial path or a Bluetooth RFCOMM
// target. Recognised RFCOMM forms:
//
//	rfcomm://AA:BB:CC:DD:EE:FF        (channel auto-discovered, else default)
//	rfcomm://AA:BB:CC:DD:EE:FF/1      (explicit channel 1)
//	AA:BB:CC:DD:EE:FF                 (bare MAC, channel auto)
//
// Anything else (e.g. /dev/rfcomm0, /dev/ttyUSB0, /dev/cu.*, COM3) is treated as
// a serial device path, preserving the bound-rfcomm and USB workflows.
func ParseURI(uri string) (Target, error) {
	raw := strings.TrimSpace(uri)
	if raw == "" {
		return Target{}, fmt.Errorf("empty connection URI")
	}

	macPart := raw
	channelPart := ""
	isRFCOMM := false

	if rest, ok := strings.CutPrefix(raw, "rfcomm://"); ok {
		isRFCOMM = true
		macPart = rest
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			macPart, channelPart = rest[:i], rest[i+1:]
		}
	}

	if !isRFCOMM && !macPattern.MatchString(raw) {
		return Target{Kind: TargetSerial, Path: raw}, nil
	}

	mac, err := parseMAC(macPart)
	if err != nil {
		return Target{}, err
	}

	var channel uint8
	if channelPart != "" {
		n, err := strconv.ParseUint(channelPart, 10, 8)
		if err != nil || n < 1 || n > 30 {
			return Target{}, fmt.Errorf("invalid RFCOMM channel %q (expected 1-30)", channelPart)
		}
		channel = uint8(n)
	}

	return Target{
		Kind:      TargetRFCOMM,
		MAC:       mac,
		MACString: strings.ToUpper(strings.ReplaceAll(macPart, "-", ":")),
		Channel:   channel,
	}, nil
}

// parseMAC parses a colon- or hyphen-separated MAC into 6 bytes in network order.
func parseMAC(s string) ([6]byte, error) {
	var mac [6]byte
	if !macPattern.MatchString(s) {
		return mac, fmt.Errorf("invalid Bluetooth MAC %q", s)
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ':' || r == '-' })
	for i, p := range parts {
		b, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return mac, fmt.Errorf("invalid MAC octet %q: %w", p, err)
		}
		mac[i] = byte(b)
	}
	return mac, nil
}

// bdaddr returns the MAC reversed into the little-endian byte order Linux
// Bluetooth sockaddrs (sockaddr_rc / sockaddr_l2) expect.
func bdaddr(mac [6]byte) [6]byte {
	var b [6]byte
	for i := range mac {
		b[i] = mac[len(mac)-1-i]
	}
	return b
}
