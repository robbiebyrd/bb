package obd

import (
	"strings"
	"sync"
	"time"
)

// scriptedTransport is an in-memory Transport that emulates the line-oriented
// ELM327/STN protocol: each command written (terminated by CR) yields a mapped
// response appended to the read queue, and a configured monitor command streams
// a canned byte script. It records every command for ordering assertions.
type scriptedTransport struct {
	mu sync.Mutex

	// responses maps a command (without trailing CR) to the bytes the device
	// returns. Unmapped commands get defaultResponse. A bare CR (the monitor
	// stop byte) returns just a prompt.
	responses       map[string]string
	defaultResponse string

	// monitorCmd, when written, appends monitorScript to the read queue instead
	// of a normal response.
	monitorCmd    string
	monitorScript string

	writeLine strings.Builder
	writes    []string
	readQ     []byte
	closed    bool
}

func newScriptedTransport() *scriptedTransport {
	return &scriptedTransport{
		responses:       map[string]string{},
		defaultResponse: "OK\r>",
	}
}

func (s *scriptedTransport) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range p {
		if b == '\r' {
			cmd := s.writeLine.String()
			s.writeLine.Reset()
			s.writes = append(s.writes, cmd)
			switch {
			case cmd == "":
				// monitor stop byte → device returns to prompt
				s.readQ = append(s.readQ, '>')
			case cmd == s.monitorCmd:
				s.readQ = append(s.readQ, []byte(s.monitorScript)...)
			default:
				resp, ok := s.responses[cmd]
				if !ok {
					resp = s.defaultResponse
				}
				s.readQ = append(s.readQ, []byte(resp)...)
			}
			continue
		}
		s.writeLine.WriteByte(b)
	}
	return len(p), nil
}

func (s *scriptedTransport) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.readQ) == 0 {
		// Emulate a serial read timeout: no data, no error.
		s.mu.Unlock()
		time.Sleep(time.Millisecond)
		s.mu.Lock()
		return 0, nil
	}
	n := copy(p, s.readQ)
	s.readQ = s.readQ[n:]
	return n, nil
}

func (s *scriptedTransport) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// commands returns a copy of the recorded command list.
func (s *scriptedTransport) commands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.writes))
	copy(out, s.writes)
	return out
}

// sawCommand reports whether cmd was written.
func (s *scriptedTransport) sawCommand(cmd string) bool {
	for _, c := range s.commands() {
		if c == cmd {
			return true
		}
	}
	return false
}
