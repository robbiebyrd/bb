package obd

import "fmt"

// STN drives an STN11xx-class device (e.g. the OBDLink MX+, STN2120). It is a
// superset of ELM327: every AT command still applies, and the ST command set
// adds multiple hardware pass filters and dedicated monitor commands that make
// passive logging — and the hybrid monitor+poll workflow — practical at the
// throughput a single Bluetooth link allows.
type STN struct {
	*ELM327
}

// NewSTN builds an ST-command driver over a transport.
func NewSTN(t Transport, proto Protocol, debug bool, logf Logf) *STN {
	return &STN{ELM327: NewELM327(t, proto, debug, logf)}
}

// Init configures the device for raw frame logging. The AT init sequence applies
// to STN devices unchanged; afterwards any stale ST filters are cleared so a
// fresh filter set can be applied via SetPassFilters.
func (s *STN) Init() error {
	if err := s.ELM327.Init(); err != nil {
		return err
	}
	if err := s.clearFilters(); err != nil {
		return err
	}
	return nil
}

// SetPassFilters installs hardware pass filters so the adapter only forwards
// frames whose identifier matches one of the given ids. Unlike ELM327's single
// code/mask, STN supports many exact filters, so each id gets its own STFAP
// entry — frames are pruned in the adapter before they ever cross the (slow)
// Bluetooth link. Passing no ids clears all filters (all frames pass).
func (s *STN) SetPassFilters(ids []uint32) error {
	if err := s.clearFilters(); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	width := 3
	if s.proto.Extended {
		width = 8
	}
	mask := uint32(0x7FF)
	if s.proto.Extended {
		mask = 0x1FFFFFFF
	}
	for _, id := range ids {
		cmd := fmt.Sprintf("STFAP %0*X,%0*X", width, id, width, mask)
		if err := s.expectOK(cmd); err != nil {
			return fmt.Errorf("adding pass filter for %X: %w", id, err)
		}
	}
	return nil
}

// clearFilters removes all ST filters.
func (s *STN) clearFilters() error {
	if err := s.expectOK("STFCA"); err != nil {
		return fmt.Errorf("clearing ST filters: %w", err)
	}
	return nil
}

// Monitor streams raw CAN frames using STM, honouring any installed pass filters
// (with none installed, STM passes all identifiers). STMA is deliberately not
// used: it reassembles CAN as ISO 15765 and overrides configured filters with a
// temporary pass-all, neither of which suits raw frame logging. Hardware pass
// filters via SetPassFilters are what keep a busy bus from overrunning a slow
// Bluetooth link.
func (s *STN) Monitor(onFrame func(Frame), onError func(error), stop <-chan struct{}) error {
	return s.Device.Monitor("STM", s.proto.Extended, onFrame, onError, stop)
}
