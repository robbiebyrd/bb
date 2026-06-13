package obd

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Frame is a single CAN frame decoded from an ELM327/STN ASCII monitor or
// response line.
type Frame struct {
	ID       uint32
	Extended bool
	Data     []byte
}

// nonFrameTokens are status strings an ELM327/STN device emits in place of, or
// alongside, frame data. They must not be parsed as frames.
var nonFrameTokens = map[string]struct{}{
	"OK":                {},
	"NO DATA":           {},
	"CAN ERROR":         {},
	"BUFFER FULL":       {},
	"BUS BUSY":          {},
	"BUS ERROR":         {},
	"DATA ERROR":        {},
	"STOPPED":           {},
	"SEARCHING":         {},
	"UNABLE TO CONNECT": {},
	"?":                 {},
	"":                  {},
}

// IsStatusToken reports whether a line is a device status message rather than a
// CAN frame. "SEARCHING..." and similar prefixes are matched as well.
func IsStatusToken(line string) bool {
	line = strings.ToUpper(strings.TrimSpace(line))
	if _, ok := nonFrameTokens[line]; ok {
		return true
	}
	// Some firmwares print "SEARCHING..." or echo banners like "ELM327 v1.5".
	if strings.HasPrefix(line, "SEARCHING") || strings.HasPrefix(line, "ELM327") || strings.HasPrefix(line, "STN") || strings.HasPrefix(line, "OBDLINK") {
		return true
	}
	return false
}

// ParseLine decodes one monitor/response line into a Frame. The line is expected
// to have headers enabled (ATH1) and spaces disabled (ATS0), so it is a run of
// hex characters: an 11-bit (3 hex digit) or 29-bit (8 hex digit) header
// followed by the frame payload. extended selects the header width.
//
// Status tokens (see IsStatusToken) and malformed lines return an error; callers
// monitoring a busy bus should skip such lines rather than treat them as fatal.
func ParseLine(line string, extended bool) (Frame, error) {
	clean := strings.ReplaceAll(strings.TrimSpace(line), " ", "")
	if IsStatusToken(clean) {
		return Frame{}, fmt.Errorf("not a frame: %q", line)
	}

	idLen := 3
	if extended {
		idLen = 8
	}

	if len(clean) < idLen {
		return Frame{}, fmt.Errorf("line %q shorter than %d-bit header", line, idLen*4)
	}

	id, err := strconv.ParseUint(clean[:idLen], 16, 32)
	if err != nil {
		return Frame{}, fmt.Errorf("invalid identifier in %q: %w", line, err)
	}

	payload := clean[idLen:]
	if len(payload)%2 != 0 {
		return Frame{}, fmt.Errorf("odd-length payload in %q", line)
	}

	data, err := hex.DecodeString(payload)
	if err != nil {
		return Frame{}, fmt.Errorf("invalid payload in %q: %w", line, err)
	}

	return Frame{ID: uint32(id), Extended: extended, Data: data}, nil
}
