package playback

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// CandumpParser parses candump log files produced by candump -ta (absolute timestamp mode).
// Each data line has the form:
//
//	(unix_seconds.fractional) interface hexID#hexData [R|T]
//
// The trailing direction flag is optional; its absence implies receive (Transmit=false).
type CandumpParser struct {
	l *slog.Logger
}

func (p *CandumpParser) Parse(path string) ([]LogEntry, error) {
	logger := p.l
	if logger == nil {
		logger = slog.Default()
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening log file: %w", err)
	}
	defer f.Close()

	var entries []LogEntry
	var baseNs int64 = -1
	var skipped int

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		entry, tsNs, err := parseCandumpLine(line)
		if err != nil {
			logger.Debug("playback: skipping unparseable line", "line", line, "error", err)
			skipped++
			continue
		}

		if baseNs < 0 {
			baseNs = tsNs
		}
		entry.OffsetNs = tsNs - baseNs
		entries = append(entries, *entry)
	}

	if skipped > 0 {
		logger.Warn("playback: skipped unparseable lines", "path", path, "skipped", skipped)
	}

	return entries, scanner.Err()
}

// parseCandumpLine parses a single candump data line and returns the entry
// and its absolute timestamp in nanoseconds.
func parseCandumpLine(line string) (*LogEntry, int64, error) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return nil, 0, fmt.Errorf("too few fields in %q", line)
	}

	// fields[0]: "(unix_seconds.fractional)"
	tsStr := strings.Trim(fields[0], "()")
	tsNs, err := parseCandumpTimestamp(tsStr)
	if err != nil {
		return nil, 0, err
	}

	// fields[1]: interface name (can0, can1, …) — ignored; one client per interface.

	// fields[3] (optional): direction flag — R=receive, T=transmit.
	transmit := len(fields) >= 4 && strings.EqualFold(fields[3], "T")

	// fields[2]: "hexID#hexData" or "hexID#R[dlc]" for remote frames.
	frameParts := strings.SplitN(fields[2], "#", 2)
	if len(frameParts) != 2 {
		return nil, 0, fmt.Errorf("invalid frame format %q", fields[2])
	}
	idPart, dataPart := frameParts[0], frameParts[1]

	id64, err := strconv.ParseUint(idPart, 16, 32)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid CAN ID %q: %w", idPart, err)
	}

	// Remote frame: data part begins with 'R'.
	remote := len(dataPart) > 0 && dataPart[0] == 'R'

	var data []byte
	if !remote {
		data, err = hex.DecodeString(dataPart)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid data %q: %w", dataPart, err)
		}
	}

	return &LogEntry{
		ID:       uint32(id64),
		Transmit: transmit,
		Remote:   remote,
		Length:   uint8(len(data)),
		Data:     data,
	}, tsNs, nil
}

// parseCandumpTimestamp converts a "seconds.fractional" string to nanoseconds.
// The fractional part is normalised to nanosecond precision regardless of how
// many decimal digits are present, avoiding float64 rounding errors.
func parseCandumpTimestamp(s string) (int64, error) {
	parts := strings.SplitN(s, ".", 2)
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid seconds in %q: %w", s, err)
	}
	if len(parts) == 1 {
		return sec * 1_000_000_000, nil
	}

	fracStr := parts[1]
	frac, err := strconv.ParseInt(fracStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid fractional seconds in %q: %w", s, err)
	}

	// Scale frac so that it represents nanoseconds.
	// fracStr is N digits → frac / 10^N seconds → frac * 10^(9-N) nanoseconds.
	n := len(fracStr)
	for i := n; i < 9; i++ {
		frac *= 10
	}
	for i := 9; i < n; i++ {
		frac /= 10
	}

	return sec*1_000_000_000 + frac, nil
}
