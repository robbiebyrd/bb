package obd

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseCANID parses a CAN identifier written in hexadecimal — the convention for
// CAN/OBD tooling — with an optional "0x" prefix ("7E8", "0x7E8", "18DAF110").
func ParseCANID(s string) (uint32, error) {
	clean := strings.TrimSpace(s)
	clean = strings.TrimPrefix(clean, "0x")
	clean = strings.TrimPrefix(clean, "0X")
	if clean == "" {
		return 0, fmt.Errorf("empty CAN ID")
	}
	v, err := strconv.ParseUint(clean, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid CAN ID %q (expected hex): %w", s, err)
	}
	return uint32(v), nil
}

// ParseCANIDs parses a list of hex CAN identifiers, skipping blank entries.
func ParseCANIDs(ss []string) ([]uint32, error) {
	out := make([]uint32, 0, len(ss))
	for _, s := range ss {
		if strings.TrimSpace(s) == "" {
			continue
		}
		id, err := ParseCANID(s)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}
