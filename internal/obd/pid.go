package obd

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// NormalizePIDRequest validates and canonicalises an OBD-II request string such
// as "01 0C" or "010c" into upper-case, space-free hex ("010C"). It returns an
// error for non-hex or odd-length input so a bad poll-list entry fails loudly at
// configuration time rather than silently producing garbage on the wire.
func NormalizePIDRequest(req string) (string, error) {
	clean := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(req), " ", ""))
	if clean == "" {
		return "", fmt.Errorf("empty PID request")
	}
	if len(clean)%2 != 0 {
		return "", fmt.Errorf("PID request %q has odd hex length", req)
	}
	if _, err := hex.DecodeString(clean); err != nil {
		return "", fmt.Errorf("PID request %q is not valid hex: %w", req, err)
	}
	return clean, nil
}

// NormalizePIDRequests normalises a list of PID requests, returning the first
// error encountered.
func NormalizePIDRequests(reqs []string) ([]string, error) {
	out := make([]string, 0, len(reqs))
	for _, r := range reqs {
		if strings.TrimSpace(r) == "" {
			continue
		}
		n, err := NormalizePIDRequest(r)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}
