package obd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestELM327_Init_CommandOrder(t *testing.T) {
	tr := newScriptedTransport()
	tr.responses["ATZ"] = "ELM327 v1.5\r>"
	elm := NewELM327(tr, ProtocolCAN11_500, false, nil)

	require.NoError(t, elm.Init())

	cmds := tr.commands()
	// Reset must come first and the protocol selector must reflect the config.
	assert.Equal(t, "ATZ", cmds[0])
	assert.True(t, tr.sawCommand("ATE0"))
	assert.True(t, tr.sawCommand("ATH1"))
	assert.True(t, tr.sawCommand("ATCAF0"))
	assert.True(t, tr.sawCommand("ATSP6"))
}

func TestELM327_Init_29BitProtocol(t *testing.T) {
	tr := newScriptedTransport()
	tr.responses["ATZ"] = "ELM327 v1.5\r>"
	elm := NewELM327(tr, ProtocolCAN29_500, false, nil)
	require.NoError(t, elm.Init())
	assert.True(t, tr.sawCommand("ATSP7"))
	assert.True(t, elm.Extended())
}

func TestELM327_SetRxFilter(t *testing.T) {
	tr := newScriptedTransport()
	elm := NewELM327(tr, ProtocolCAN11_500, false, nil)

	require.NoError(t, elm.SetRxFilter([]uint32{0x7E8}))
	assert.True(t, tr.sawCommand("ATCF7E8"))
	assert.True(t, tr.sawCommand("ATCM7FF"))
}

func TestELM327_SetRxFilter_EmptyClears(t *testing.T) {
	tr := newScriptedTransport()
	elm := NewELM327(tr, ProtocolCAN11_500, false, nil)

	require.NoError(t, elm.SetRxFilter(nil))
	assert.True(t, tr.sawCommand("ATCM000"))
}

func TestELM327_Request(t *testing.T) {
	tr := newScriptedTransport()
	// Engine RPM response from one ECU.
	tr.responses["010C"] = "7E8041A2B3C\r>"
	elm := NewELM327(tr, ProtocolCAN11_500, false, nil)

	frames, err := elm.Request("01 0c")
	require.NoError(t, err)
	require.Len(t, frames, 1)
	assert.Equal(t, uint32(0x7E8), frames[0].ID)
	assert.True(t, tr.sawCommand("010C"))
}

func TestCombinedFilter(t *testing.T) {
	// A single id yields an exact match.
	code, mask := combinedFilter([]uint32{0x7E8})
	assert.Equal(t, uint32(0x7E8), code)
	assert.Equal(t, uint32(0x7FF), mask)

	// Two ids that differ only in the low nibble share the upper bits.
	code, mask = combinedFilter([]uint32{0x7E8, 0x7E9})
	assert.Equal(t, uint32(0x7E8), code)
	// Bit 0 varies, so its mask bit is cleared.
	assert.Equal(t, uint32(0x7FE), mask)
}
