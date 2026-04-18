package playback

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LogEntry is a single CAN frame with its playback timing offset from the
// first frame in the file.
type LogEntry struct {
	OffsetNs int64  // nanoseconds since the first frame
	ID       uint32
	Transmit bool
	Remote   bool
	Length   uint8
	Data     []byte
}

// LogParser parses a log file into a sequence of timed CAN frames.
// New formats can be supported by implementing this interface.
type LogParser interface {
	Parse(path string) ([]LogEntry, error)
}

// DetectParser inspects the file header and returns the appropriate LogParser.
func DetectParser(path string) (LogParser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "***BUSMASTER"):
			return &BUSMASTERParser{}, nil
		case strings.HasPrefix(line, "("):
			return &CandumpParser{}, nil
		}
	}
	return nil, fmt.Errorf("unsupported log format: %s", path)
}

// BUSMASTERParser parses BUSMASTER ver 3.x .log files in ABSOLUTE MODE.
// Each line outside of *** header blocks has the form:
//
//	H:M:S:MMMM  Tx/Rx  Channel  0xID  Type  DLC  [DataBytes...]
type BUSMASTERParser struct{}

func (p *BUSMASTERParser) Parse(path string) ([]LogEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening log file: %w", err)
	}
	defer f.Close()

	var entries []LogEntry
	var baseNs int64 = -1

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "***") {
			continue
		}

		entry, tsNs, err := parseBUSMASTERLine(line)
		if err != nil {
			continue // skip unparseable lines silently
		}

		if baseNs < 0 {
			baseNs = tsNs
		}
		entry.OffsetNs = tsNs - baseNs
		entries = append(entries, *entry)
	}

	return entries, scanner.Err()
}

// parseBUSMASTERLine parses a single data line and returns the entry and its
// absolute timestamp in nanoseconds.
func parseBUSMASTERLine(line string) (*LogEntry, int64, error) {
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return nil, 0, fmt.Errorf("too few fields in %q", line)
	}

	tsNs, err := parseBUSMASTERTimestamp(fields[0])
	if err != nil {
		return nil, 0, err
	}

	transmit := fields[1] == "Tx"

	idStr := strings.TrimPrefix(strings.TrimPrefix(fields[3], "0x"), "0X")
	id64, err := strconv.ParseUint(idStr, 16, 32)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid CAN ID %q: %w", fields[3], err)
	}

	frameType := strings.ToLower(fields[4])
	remote := strings.Contains(frameType, "r")

	dlc, err := strconv.ParseUint(fields[5], 10, 8)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid DLC %q: %w", fields[5], err)
	}

	data := make([]byte, 0, dlc)
	for i := 6; i < 6+int(dlc) && i < len(fields); i++ {
		b, err := strconv.ParseUint(fields[i], 16, 8)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid data byte %q: %w", fields[i], err)
		}
		data = append(data, byte(b))
	}

	return &LogEntry{
		ID:       uint32(id64),
		Transmit: transmit,
		Remote:   remote,
		Length:   uint8(dlc),
		Data:     data,
	}, tsNs, nil
}

// parseBUSMASTERTimestamp converts a "H:M:S:MMMM" string to nanoseconds.
func parseBUSMASTERTimestamp(s string) (int64, error) {
	parts := strings.SplitN(s, ":", 4)
	if len(parts) != 4 {
		return 0, fmt.Errorf("invalid timestamp %q: expected H:M:S:MMMM", s)
	}

	h, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid hours in %q: %w", s, err)
	}
	m, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid minutes in %q: %w", s, err)
	}
	sec, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid seconds in %q: %w", s, err)
	}
	ms, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid milliseconds in %q: %w", s, err)
	}

	return (h*3600+m*60+sec)*1_000_000_000 + ms*1_000_000, nil
}
