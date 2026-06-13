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
	// AT base init still runs, plus ST pass-filter clear.
	assert.True(t, tr.sawCommand("ATSP6"))
	assert.True(t, tr.sawCommand("STFPC"))
}

func TestSTN_SetPassFilters(t *testing.T) {
	tr := newScriptedTransport()
	stn := NewSTN(tr, ProtocolCAN11_500, false, nil)

	require.NoError(t, stn.SetPassFilters([]uint32{0x7E8, 0x244}))
	assert.True(t, tr.sawCommand("STFPA 07E8,07FF"))
	assert.True(t, tr.sawCommand("STFPA 0244,07FF"))
}

func TestSTN_SetPassFilters_Extended(t *testing.T) {
	tr := newScriptedTransport()
	stn := NewSTN(tr, ProtocolCAN29_500, false, nil)

	require.NoError(t, stn.SetPassFilters([]uint32{0x18DAF110}))
	assert.True(t, tr.sawCommand("STFPA 18DAF110,1FFFFFFF"))
}

func TestSTN_StartPeriodic(t *testing.T) {
	tr := newScriptedTransport()
	// STPPMA returns a hex handle rather than OK.
	tr.responses["STPPMA 500, 7DF, 02010C"] = "1\r>"
	tr.responses["STPPMA 500, 7DF, 02010D"] = "2\r>"
	stn := NewSTN(tr, ProtocolCAN11_500, false, nil)

	require.NoError(t, stn.StartPeriodic([]string{"010C", "010D"}, 500*time.Millisecond))
	// Normal monitoring mode must be enabled so periodic frames are transmitted.
	assert.True(t, tr.sawCommand("STCMM 1"))
	// Each PID becomes an ISO-TP single frame (PCI length byte + service bytes).
	assert.True(t, tr.sawCommand("STPPMA 500, 7DF, 02010C"))
	assert.True(t, tr.sawCommand("STPPMA 500, 7DF, 02010D"))
}

func TestSTN_StartPeriodic_OutOfMemoryFails(t *testing.T) {
	tr := newScriptedTransport()
	tr.responses["STPPMA 1000, 7DF, 02010C"] = "OUT OF MEMORY\r>"
	stn := NewSTN(tr, ProtocolCAN11_500, false, nil)

	err := stn.StartPeriodic([]string{"010C"}, time.Second)
	require.Error(t, err)
}

func TestSTN_StopPeriodic(t *testing.T) {
	tr := newScriptedTransport()
	stn := NewSTN(tr, ProtocolCAN11_500, false, nil)

	require.NoError(t, stn.StopPeriodic())
	assert.True(t, tr.sawCommand("STPPMC"))
	assert.True(t, tr.sawCommand("STCMM 0"))
}

func TestObdSingleFrame(t *testing.T) {
	got, err := obdSingleFrame("010C")
	require.NoError(t, err)
	assert.Equal(t, "02010C", got)

	got, err = obdSingleFrame("01 0d")
	require.NoError(t, err)
	assert.Equal(t, "02010D", got)
}

func TestSTN_Monitor_UsesSTMWithoutFilters(t *testing.T) {
	tr := newScriptedTransport()
	tr.monitorCmd = "STM"
	tr.monitorScript = "7E80100\r"
	stn := NewSTN(tr, ProtocolCAN11_500, false, nil)

	runMonitorOnce(t, func(onFrame func(Frame), stop <-chan struct{}) error {
		return stn.Monitor(onFrame, nil, stop)
	})
	// STM (raw) is used in all cases; STMA would reassemble as ISO 15765.
	assert.True(t, tr.sawCommand("STM"))
	assert.False(t, tr.sawCommand("STMA"))
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
