package obd2

// DecodePIDsSupported decodes an OBD-II "PIDs supported" bitmask into a sorted
// list of supported PID numbers.
//
// The bitmask is a 32-bit big-endian value where the MSB (bit 31) corresponds
// to base+1 and the LSB (bit 0) corresponds to base+32. For example, PID 0x00
// returns a bitmask for PIDs 0x01–0x20 (base=0x00), PID 0x20 returns a bitmask
// for PIDs 0x21–0x40 (base=0x20), and so on.
//
// The value parameter accepts a uint32 (or float64 truncated to uint32) as
// produced by the DBC parser for signals like S01PID00_PIDsSupported_01_20.
func DecodePIDsSupported(bitmask uint32, base uint8) []uint8 {
	var pids []uint8
	for i := 0; i < 32; i++ {
		if bitmask&(1<<(31-i)) != 0 {
			pids = append(pids, base+uint8(i)+1)
		}
	}
	return pids
}

// pidsSupportedSignals maps DBC signal names to their PID base values.
var pidsSupportedSignals = map[string]uint8{
	"S01PID00_PIDsSupported_01_20": 0x00,
	"S01PID20_PIDsSupported_21_40": 0x20,
	"S01PID40_PIDsSupported_41_60": 0x40,
	"S01PID60_PIDsSupported_61_80": 0x60,
	"S01PID80_PIDsSupported_81_A0": 0x80,
	"S01PIDA0_PIDsSupported_A1_C0": 0xA0,
	"S01PIDC0_PIDsSupported_C1_E0": 0xC0,
}

// IsPIDsSupportedSignal checks whether a DBC signal name is one of the OBD-II
// "PIDs supported" bitmask signals. If so, it returns the base PID for that
// range and true; otherwise it returns 0 and false.
func IsPIDsSupportedSignal(signal string) (uint8, bool) {
	base, ok := pidsSupportedSignals[signal]
	return base, ok
}
