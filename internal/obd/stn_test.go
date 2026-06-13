package obd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSTN_Init_ClearsFilters(t *testing.T) {
	tr := newScriptedTransport()
	tr.responses["ATZ"] = "STN2120 v5.5.2\r>"
	stn := NewSTN(tr, ProtocolCAN11_500, false, nil)

	require.NoError(t, stn.Init())
	// AT base init still runs, plus ST filter clear.
	assert.True(t, tr.sawCommand("ATSP6"))
	assert.True(t, tr.sawCommand("STFCA"))
}

func TestSTN_SetPassFilters(t *testing.T) {
	tr := newScriptedTransport()
	stn := NewSTN(tr, ProtocolCAN11_500, false, nil)

	require.NoError(t, stn.SetPassFilters([]uint32{0x7E8, 0x244}))
	assert.True(t, tr.sawCommand("STFAP 7E8,7FF"))
	assert.True(t, tr.sawCommand("STFAP 244,7FF"))
}

func TestSTN_SetPassFilters_Extended(t *testing.T) {
	tr := newScriptedTransport()
	stn := NewSTN(tr, ProtocolCAN29_500, false, nil)

	require.NoError(t, stn.SetPassFilters([]uint32{0x18DAF110}))
	assert.True(t, tr.sawCommand("STFAP 18DAF110,1FFFFFFF"))
}

func TestSTN_Monitor_UsesSTMAWithoutFilters(t *testing.T) {
	tr := newScriptedTransport()
	tr.monitorCmd = "STMA"
	tr.monitorScript = "7E80100\r"
	stn := NewSTN(tr, ProtocolCAN11_500, false, nil)

	runMonitorOnce(t, func(onFrame func(Frame), stop <-chan struct{}) error {
		return stn.Monitor(onFrame, nil, stop)
	})
	assert.True(t, tr.sawCommand("STMA"))
}

func TestSTN_Monitor_UsesSTMWithFilters(t *testing.T) {
	tr := newScriptedTransport()
	tr.monitorCmd = "STM"
	tr.monitorScript = "7E80100\r"
	stn := NewSTN(tr, ProtocolCAN11_500, false, nil)
	require.NoError(t, stn.SetPassFilters([]uint32{0x7E8}))

	runMonitorOnce(t, func(onFrame func(Frame), stop <-chan struct{}) error {
		return stn.Monitor(onFrame, nil, stop)
	})
	assert.True(t, tr.sawCommand("STM"))
}

// runMonitorOnce starts a monitor function, waits for one frame, then stops it.
func runMonitorOnce(t *testing.T, run func(onFrame func(Frame), stop <-chan struct{}) error) {
	t.Helper()
	frames := make(chan Frame, 4)
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- run(func(f Frame) { frames <- f }, stop)
	}()
	select {
	case <-frames:
	case <-time.After(2 * time.Second):
		t.Fatal("no frame received")
	}
	close(stop)
	require.NoError(t, <-done)
}
