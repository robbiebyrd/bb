package obd

import (
	"fmt"
	"strings"
)

// ELM327 drives a device using the base AT command set. STN devices also
// understand every command here; the STN type embeds this and layers ST
// extensions on top.
type ELM327 struct {
	*Device
	proto Protocol
}

// NewELM327 builds an AT-command driver over a transport.
func NewELM327(t Transport, proto Protocol, debug bool, logf Logf) *ELM327 {
	return &ELM327{Device: NewDevice(t, debug, logf), proto: proto}
}

// Protocol returns the configured protocol descriptor.
func (e *ELM327) Protocol() Protocol { return e.proto }

// Extended reports whether the configured protocol uses 29-bit identifiers.
func (e *ELM327) Extended() bool { return e.proto.Extended }

// Init resets the adapter and configures it for raw frame logging: echo off,
// linefeeds/spaces off (so monitor lines are pure hex), headers on, CAN
// auto-formatting off, and the requested protocol.
func (e *ELM327) Init() error {
	// Reset; the response is a version banner rather than OK.
	if _, err := e.command("ATZ"); err != nil {
		return fmt.Errorf("resetting adapter: %w", err)
	}

	for _, cmd := range []string{
		"ATE0",                    // echo off
		"ATL0",                    // linefeeds off
		"ATS0",                    // spaces off — monitor lines become contiguous hex
		"ATH1",                    // headers on — include the CAN identifier in every line
		"ATCAF0",                  // CAN auto-formatting off — emit raw frames, no PCI handling
		"ATCFC0",                  // CAN flow control off — we are passively monitoring
		"ATSP" + e.proto.Selector, // set protocol
	} {
		if err := e.expectOK(cmd); err != nil {
			return fmt.Errorf("init command %q: %w", cmd, err)
		}
	}
	return nil
}

// SetRxFilter restricts which frames the adapter passes through to those whose
// identifier matches the given ids. ELM327 supports a single code/mask pair, so
// multiple ids are reduced to the tightest common code+mask (the same scheme the
// adapter firmware uses). Passing no ids clears the filter (all frames pass).
func (e *ELM327) SetRxFilter(ids []uint32) error {
	if len(ids) == 0 {
		return e.ClearRxFilter()
	}
	code, mask := combinedFilter(ids)
	if err := e.expectOK(fmt.Sprintf("ATCF%03X", code)); err != nil {
		return fmt.Errorf("setting CAN code filter: %w", err)
	}
	if err := e.expectOK(fmt.Sprintf("ATCM%03X", mask)); err != nil {
		return fmt.Errorf("setting CAN mask filter: %w", err)
	}
	return nil
}

// ClearRxFilter removes any receive filter so all frames are monitored.
func (e *ELM327) ClearRxFilter() error {
	// A mask of all-zero accepts every identifier.
	if err := e.expectOK("ATCM000"); err != nil {
		return fmt.Errorf("clearing CAN filter: %w", err)
	}
	return nil
}

// MonitorAll streams every frame the adapter receives (subject to any RX filter)
// using ATMA, until stop is signalled.
func (e *ELM327) MonitorAll(onFrame func(Frame), onError func(error), stop <-chan struct{}) error {
	return e.Monitor("ATMA", e.proto.Extended, onFrame, onError, stop)
}

// Request sends an OBD-II service request (e.g. "010C" for engine RPM) to the
// default functional address and returns the raw response frames. With headers
// on and auto-formatting off, each response line is a raw CAN frame, so callers
// get the same Frame shape as monitor mode and can decode downstream.
func (e *ELM327) Request(serviceHex string) ([]Frame, error) {
	cmd := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(serviceHex), " ", ""))
	resp, err := e.command(cmd)
	if err != nil {
		return nil, err
	}
	return parseResponseFrames(resp, e.proto.Extended), nil
}

// parseResponseFrames splits a command response into frames, skipping status
// lines such as "SEARCHING..." and "NO DATA".
func parseResponseFrames(resp string, extended bool) []Frame {
	var frames []Frame
	for _, raw := range strings.Split(resp, " ") {
		line := strings.TrimSpace(raw)
		if line == "" || IsStatusToken(line) {
			continue
		}
		if frame, err := ParseLine(line, extended); err == nil {
			frames = append(frames, frame)
		}
	}
	return frames
}

// combinedFilter reduces a set of identifiers to a single ELM327 code/mask pair
// that passes at least those identifiers. This mirrors the adapter's own filter
// math: the code is the AND of all ids, the mask marks the bits they share.
func combinedFilter(ids []uint32) (code uint32, mask uint32) {
	code = 0x7FF
	var orAll uint32
	for _, id := range ids {
		code &= id
		orAll |= id
	}
	// Mask bits set where all ids agree (i.e. the bits that did not vary).
	mask = (^(orAll ^ code)) & 0x7FF
	return code, mask
}
