package obd

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// prompt is the character an ELM327/STN device sends when it is ready for the
// next command.
const prompt = '>'

// carriageReturn terminates commands and separates response lines.
const carriageReturn = '\r'

// defaultCommandTimeout bounds how long command() waits for a prompt.
const defaultCommandTimeout = 2 * time.Second

// Logf is an optional debug logging hook.
type Logf func(format string, args ...any)

// Device is the transport-agnostic ELM327/STN protocol engine. It implements the
// line-oriented "command, read until prompt" protocol shared by the AT and ST
// command sets, plus a streaming monitor loop. It is not safe for concurrent use
// by multiple goroutines; a single owner goroutine should drive it.
type Device struct {
	t     Transport
	debug bool
	logf  Logf
	// readBuf is reused across reads to avoid per-read allocation.
	readBuf [256]byte
	// pending holds bytes read past a prompt/CR boundary so the next read picks
	// up where the last one left off.
	pending []byte
}

// NewDevice wraps a transport in a protocol engine.
func NewDevice(t Transport, debug bool, logf Logf) *Device {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Device{t: t, debug: debug, logf: logf}
}

// Close closes the underlying transport.
func (d *Device) Close() error { return d.t.Close() }

// writeLine writes a command followed by a carriage return.
func (d *Device) writeLine(cmd string) error {
	if d.debug {
		d.logf(">> %s", cmd)
	}
	buf := append([]byte(cmd), carriageReturn)
	n, err := d.t.Write(buf)
	if err != nil {
		return fmt.Errorf("writing %q: %w", cmd, err)
	}
	if n != len(buf) {
		return fmt.Errorf("short write for %q: %d/%d bytes", cmd, n, len(buf))
	}
	return nil
}

// command sends cmd and returns the response with the trailing prompt and
// surrounding carriage returns stripped.
func (d *Device) command(cmd string) (string, error) {
	return d.commandTimeout(cmd, defaultCommandTimeout)
}

func (d *Device) commandTimeout(cmd string, timeout time.Duration) (string, error) {
	if err := d.writeLine(cmd); err != nil {
		return "", err
	}
	resp, err := d.readUntilPrompt(timeout)
	if err != nil {
		return "", fmt.Errorf("command %q: %w", cmd, err)
	}
	if d.debug {
		d.logf("<< %s", strings.ReplaceAll(resp, "\r", " "))
	}
	return resp, nil
}

// readUntilPrompt reads from the transport until the '>' prompt is seen or the
// timeout elapses. The returned string excludes the prompt.
func (d *Device) readUntilPrompt(timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	var sb strings.Builder

	// Drain any bytes left from a previous read first.
	if len(d.pending) > 0 {
		if idx := indexByte(d.pending, prompt); idx >= 0 {
			sb.Write(d.pending[:idx])
			d.pending = trimLeftCopy(d.pending[idx+1:])
			return cleanResponse(sb.String()), nil
		}
		sb.Write(d.pending)
		d.pending = nil
	}

	for {
		if time.Now().After(deadline) {
			return "", errors.New("timeout waiting for prompt")
		}
		n, err := d.t.Read(d.readBuf[:])
		if n > 0 {
			chunk := d.readBuf[:n]
			if idx := indexByte(chunk, prompt); idx >= 0 {
				sb.Write(chunk[:idx])
				// Stash anything after the prompt for the next read.
				if idx+1 < len(chunk) {
					d.pending = append(d.pending[:0:0], chunk[idx+1:]...)
				}
				return cleanResponse(sb.String()), nil
			}
			sb.Write(chunk)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", io.EOF
			}
			return "", fmt.Errorf("reading from transport: %w", err)
		}
	}
}

// Monitor streams CAN frames produced by monitorCmd (e.g. "ATMA" or "STM") to
// onFrame until stop is signalled, the transport errors, or EOF. On stop it
// sends a single byte to break the device out of monitor mode and drains back to
// the prompt so the device is ready for the next command.
//
// Status lines (NO DATA, BUFFER FULL, banners, …) are skipped silently; only
// genuine frame-parse failures are reported via onError (when non-nil).
func (d *Device) Monitor(
	monitorCmd string,
	extended bool,
	onFrame func(Frame),
	onError func(error),
	stop <-chan struct{},
) error {
	if err := d.writeLine(monitorCmd); err != nil {
		return err
	}

	var line strings.Builder
	emit := func() {
		s := line.String()
		line.Reset()
		if strings.TrimSpace(s) == "" {
			return
		}
		if IsStatusToken(s) {
			return
		}
		frame, err := ParseLine(s, extended)
		if err != nil {
			if onError != nil {
				onError(err)
			}
			return
		}
		onFrame(frame)
	}

	// Process bytes left over from init/command reads first.
	if len(d.pending) > 0 {
		d.feedMonitor(d.pending, &line, emit)
		d.pending = nil
	}

	for {
		select {
		case <-stop:
			return d.stopMonitor()
		default:
		}

		n, err := d.t.Read(d.readBuf[:])
		if n > 0 {
			d.feedMonitor(d.readBuf[:n], &line, emit)
		}
		if err != nil {
			// Surface EOF (peer closed the link) as an error so the connection
			// layer can reconnect; a clean stop returns via the stop channel above.
			if errors.Is(err, io.EOF) {
				return io.EOF
			}
			return fmt.Errorf("monitor read: %w", err)
		}
	}
}

// feedMonitor splits a chunk into CR-delimited lines, emitting completed ones. A
// prompt byte mid-stream (the device aborting monitor) flushes the current line.
func (d *Device) feedMonitor(chunk []byte, line *strings.Builder, emit func()) {
	for _, b := range chunk {
		switch b {
		case carriageReturn, prompt:
			emit()
		case '\n':
			// ignore linefeeds
		default:
			line.WriteByte(b)
		}
	}
}

// stopMonitor breaks the device out of monitor mode and drains to the prompt.
func (d *Device) stopMonitor() error {
	// Any byte interrupts ELM327/STN monitor mode; a CR is the conventional one.
	if _, err := d.t.Write([]byte{carriageReturn}); err != nil {
		return fmt.Errorf("stopping monitor: %w", err)
	}
	// Drain until prompt so the next command starts clean. Ignore timeout: the
	// device may have already returned to prompt.
	_, _ = d.readUntilPrompt(defaultCommandTimeout)
	return nil
}

// expectOK sends a configuration command and verifies the adapter acknowledged
// it with "OK". Every command routed through here (echo/format toggles, protocol
// selection, filter and monitoring-mode setup) replies "OK" on success, so any
// other response — "?", "CAN ERROR", "UNABLE TO CONNECT", "NO DATA", … — is
// treated as a failure. This surfaces a mis-configuration at Init rather than
// letting the adapter silently run in the wrong state. (Commands that return a
// value rather than "OK", such as ATZ's banner or STPPMA's handle, use command
// directly instead of expectOK.)
func (d *Device) expectOK(cmd string) error {
	resp, err := d.command(cmd)
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToUpper(resp), "OK") {
		return nil
	}
	return fmt.Errorf("command %q returned %q, expected OK", cmd, resp)
}

// --- small helpers (avoid bytes import churn) ---

func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}

func trimLeftCopy(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// cleanResponse strips carriage returns and trims whitespace from a raw response.
func cleanResponse(s string) string {
	return strings.Trim(strings.ReplaceAll(s, "\r", " "), " \r\n")
}
