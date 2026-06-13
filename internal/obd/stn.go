package obd

import (
	"fmt"
	"strings"
	"time"
)

// STN drives an STN11xx-class device (e.g. the OBDLink MX+, STN2120). It is a
// superset of ELM327: every AT command still applies, and the ST command set
// adds multiple hardware pass filters and a firmware periodic-messaging system
// that lets the device poll PIDs in the background while monitoring — the basis
// for gap-free hybrid logging.
type STN struct {
	*ELM327
}

// obdFunctionalID is the standard OBD-II functional request identifier.
const obdFunctionalID = "7DF"

// NewSTN builds an ST-command driver over a transport.
func NewSTN(t Transport, proto Protocol, debug bool, logf Logf) *STN {
	return &STN{ELM327: NewELM327(t, proto, debug, logf)}
}

// Init configures the device for raw frame logging. The AT init sequence applies
// to STN devices unchanged; afterwards any stale ST pass filters are cleared so a
// fresh filter set can be applied via SetPassFilters.
func (s *STN) Init() error {
	if err := s.ELM327.Init(); err != nil {
		return err
	}
	return s.clearFilters()
}

// SetPassFilters installs hardware pass filters so the adapter only forwards
// frames whose identifier matches one of the given ids. Unlike ELM327's single
// code/mask, STN supports many exact filters, so each id gets its own STFPA
// entry — frames are pruned in the adapter before they ever cross the (slow)
// Bluetooth link. Passing no ids clears all pass filters (all frames pass).
func (s *STN) SetPassFilters(ids []uint32) error {
	if err := s.clearFilters(); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	width := 4
	mask := uint32(0x7FF)
	if s.proto.Extended {
		width = 8
		mask = 0x1FFFFFFF
	}
	for _, id := range ids {
		cmd := fmt.Sprintf("STFPA %0*X,%0*X", width, id, width, mask)
		if err := s.expectOK(cmd); err != nil {
			return fmt.Errorf("adding pass filter for %X: %w", id, err)
		}
	}
	return nil
}

// clearFilters removes all ST pass filters.
func (s *STN) clearFilters() error {
	if err := s.expectOK("STFPC"); err != nil {
		return fmt.Errorf("clearing ST pass filters: %w", err)
	}
	return nil
}

// Monitor streams raw CAN frames using STM, honouring any installed pass filters
// (with none installed, STM passes all identifiers). STMA is deliberately not
// used: it reassembles CAN as ISO 15765 and overrides configured filters with a
// temporary pass-all, neither of which suits raw frame logging.
func (s *STN) Monitor(onFrame func(Frame), onError func(error), stop <-chan struct{}) error {
	return s.Device.Monitor("STM", s.proto.Extended, onFrame, onError, stop)
}

// StartPeriodic offloads PID polling to the device firmware: each request is
// transmitted automatically every `period` while monitoring continues
// uninterrupted, which is what makes STN hybrid mode gap-free. STCMM 1 puts the
// node in normal (ACK-capable) mode so the periodic frames are actually sent
// during STM monitoring; STPPMA then installs one periodic message per PID, and
// the responses arrive in the monitor stream like any other frame.
func (s *STN) StartPeriodic(pids []string, period time.Duration) error {
	if err := s.expectOK("STCMM 1"); err != nil {
		return fmt.Errorf("enabling normal monitoring mode: %w", err)
	}
	ms := int(period / time.Millisecond)
	if ms <= 0 {
		ms = 1000
	}
	for _, pid := range pids {
		data, err := obdSingleFrame(pid)
		if err != nil {
			return err
		}
		cmd := fmt.Sprintf("STPPMA %d, %s, %s", ms, obdFunctionalID, data)
		resp, err := s.command(cmd)
		if err != nil {
			return fmt.Errorf("adding periodic message for %q: %w", pid, err)
		}
		if isErrorResponse(resp) {
			return fmt.Errorf("adding periodic message for %q: %q", pid, resp)
		}
	}
	return nil
}

// StopPeriodic clears all periodic messages and returns the device to silent
// monitoring.
func (s *STN) StopPeriodic() error {
	if err := s.expectOK("STPPMC"); err != nil {
		return fmt.Errorf("clearing periodic messages: %w", err)
	}
	if err := s.expectOK("STCMM 0"); err != nil {
		return fmt.Errorf("restoring silent monitoring: %w", err)
	}
	return nil
}

// obdSingleFrame wraps an OBD service request (e.g. "010C") in an ISO-TP single
// frame: a leading PCI byte giving the payload length followed by the service
// bytes (e.g. "02010C"). The leading length byte is required because the adapter
// runs with CAN auto-formatting off (ATCAF0), so it does not add one itself.
func obdSingleFrame(pid string) (string, error) {
	clean := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(pid), " ", ""))
	if clean == "" || len(clean)%2 != 0 {
		return "", fmt.Errorf("invalid PID request %q", pid)
	}
	return fmt.Sprintf("%02X%s", len(clean)/2, clean), nil
}

// isErrorResponse reports whether a device response indicates a failure rather
// than a value or acknowledgement.
func isErrorResponse(resp string) bool {
	up := strings.ToUpper(resp)
	return strings.Contains(up, "?") || strings.Contains(up, "ERROR") || strings.Contains(up, "OUT OF MEMORY")
}
