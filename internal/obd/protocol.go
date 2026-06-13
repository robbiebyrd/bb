package obd

import "strings"

// Protocol describes an OBD/CAN protocol the adapter should use. The selector
// is the argument given to the ELM327 "ATSP" command (also understood by STN
// devices), and Extended records whether the protocol uses 29-bit identifiers
// so monitor output can be parsed with the correct header width.
type Protocol struct {
	// Selector is the ATSP argument, e.g. "6" for ISO 15765-4 CAN 11-bit/500k,
	// or "0" for automatic protocol detection.
	Selector string
	// Extended is true when the protocol uses 29-bit CAN identifiers.
	Extended bool
	// Label is a human-readable description.
	Label string
}

// Well-known protocols. The selector values match the ELM327 ATSP table and are
// accepted verbatim by STN11xx devices.
var (
	ProtocolAuto      = Protocol{Selector: "0", Extended: false, Label: "Automatic"}
	ProtocolCAN11_500 = Protocol{Selector: "6", Extended: false, Label: "ISO 15765-4 CAN 11-bit/500k"}
	ProtocolCAN29_500 = Protocol{Selector: "7", Extended: true, Label: "ISO 15765-4 CAN 29-bit/500k"}
	ProtocolCAN11_250 = Protocol{Selector: "8", Extended: false, Label: "ISO 15765-4 CAN 11-bit/250k"}
	ProtocolCAN29_250 = Protocol{Selector: "9", Extended: true, Label: "ISO 15765-4 CAN 29-bit/250k"}
)

// knownProtocols maps an ATSP selector to its descriptor. Selectors are kept as
// strings because ELM327 accepts a hex digit (e.g. "A", "B", "C").
var knownProtocols = map[string]Protocol{
	"0": ProtocolAuto,
	"6": ProtocolCAN11_500,
	"7": ProtocolCAN29_500,
	"8": ProtocolCAN11_250,
	"9": ProtocolCAN29_250,
}

// ProtocolFor resolves a selector string (the ATSP argument) into a Protocol.
// Unknown selectors are accepted and assumed to use 11-bit identifiers, so a
// user can pass any value supported by their adapter; an empty selector yields
// the default ISO 15765-4 CAN 11-bit/500k protocol used by most modern vehicles.
func ProtocolFor(selector string) Protocol {
	selector = strings.TrimSpace(strings.ToUpper(selector))
	if selector == "" {
		return ProtocolCAN11_500
	}
	if p, ok := knownProtocols[selector]; ok {
		return p
	}
	return Protocol{Selector: selector, Extended: false, Label: "Custom protocol " + selector}
}
