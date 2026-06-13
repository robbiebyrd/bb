package obd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDevice_Command(t *testing.T) {
	tr := newScriptedTransport()
	tr.responses["ATI"] = "ELM327 v1.5\r>"
	dev := NewDevice(tr, false, nil)

	resp, err := dev.command("ATI")
	require.NoError(t, err)
	assert.Equal(t, "ELM327 v1.5", resp)
	assert.True(t, tr.sawCommand("ATI"))
}

func TestDevice_CommandTimeout(t *testing.T) {
	tr := newScriptedTransport()
	// Map to a response with no prompt so readUntilPrompt times out.
	tr.responses["ATX"] = "GARBAGE\r"
	dev := NewDevice(tr, false, nil)

	_, err := dev.commandTimeout("ATX", 50*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestDevice_Monitor_StreamsAndStops(t *testing.T) {
	tr := newScriptedTransport()
	tr.monitorCmd = "ATMA"
	// Three frames plus a status line that must be skipped.
	tr.monitorScript = "7E80100\rNO DATA\r2440200\r7DF0355\r"
	dev := NewDevice(tr, false, nil)

	frames := make(chan Frame, 16)
	stop := make(chan struct{})

	done := make(chan error, 1)
	go func() {
		done <- dev.Monitor("ATMA", false, func(f Frame) { frames <- f }, nil, stop)
	}()

	got := make([]Frame, 0, 3)
	timeout := time.After(2 * time.Second)
	for len(got) < 3 {
		select {
		case f := <-frames:
			got = append(got, f)
		case <-timeout:
			t.Fatalf("only received %d frames", len(got))
		}
	}
	close(stop)
	require.NoError(t, <-done)

	assert.Equal(t, uint32(0x7E8), got[0].ID)
	assert.Equal(t, uint32(0x244), got[1].ID)
	assert.Equal(t, uint32(0x7DF), got[2].ID)
	// Monitor must have written the stop byte (a bare CR) on shutdown.
	assert.True(t, tr.sawCommand(""))
}
