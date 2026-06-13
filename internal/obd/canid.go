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

// OBDResponseIDs returns the ISO 15765-4 diagnostic response identifiers an ECU
// answers OBD-II requests on. For 11-bit addressing these are 0x7E8–0x7EF; for
// 29-bit they are physical replies 0x18DAF1xx (the common low ECU addresses).
// These must pass any hardware filter when polling, or replies are dropped.
func OBDResponseIDs(extended bool) []uint32 {
	if extended {
		ids := make([]uint32, 0, 16)
		for ecu := uint32(0); ecu <= 0x0F; ecu++ {
			ids = append(ids, 0x18DAF100|ecu)
		}
		return ids
	}
	ids := make([]uint32, 0, 8)
	for id := uint32(0x7E8); id <= 0x7EF; id++ {
		ids = append(ids, id)
	}
	return ids
}

// MergeUniqueIDs concatenates CAN ID lists, preserving order and dropping
// duplicates.
func MergeUniqueIDs(lists ...[]uint32) []uint32 {
	seen := make(map[uint32]struct{})
	var out []uint32
	for _, list := range lists {
		for _, id := range list {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
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
