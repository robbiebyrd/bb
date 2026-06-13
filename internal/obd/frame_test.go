package obd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLine_11Bit(t *testing.T) {
	frame, err := ParseLine("7E80123456789ABCDEF", false)
	require.NoError(t, err)
	assert.Equal(t, uint32(0x7E8), frame.ID)
	assert.False(t, frame.Extended)
	assert.Equal(t, []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF}, frame.Data)
}

func TestParseLine_29Bit(t *testing.T) {
	frame, err := ParseLine("18DAF1100123456789ABCDEF", true)
	require.NoError(t, err)
	assert.Equal(t, uint32(0x18DAF110), frame.ID)
	assert.True(t, frame.Extended)
	assert.Len(t, frame.Data, 8)
}

func TestParseLine_StripsSpaces(t *testing.T) {
	frame, err := ParseLine("7E8 01 23 45", false)
	require.NoError(t, err)
	assert.Equal(t, uint32(0x7E8), frame.ID)
	assert.Equal(t, []byte{0x01, 0x23, 0x45}, frame.Data)
}

func TestParseLine_StatusTokensRejected(t *testing.T) {
	for _, tok := range []string{"NO DATA", "BUFFER FULL", "STOPPED", "SEARCHING...", "?", "ELM327 v1.5", ""} {
		_, err := ParseLine(tok, false)
		assert.Error(t, err, "expected %q to be rejected", tok)
	}
}

func TestParseLine_Malformed(t *testing.T) {
	// Odd-length payload.
	_, err := ParseLine("7E8123", false)
	assert.Error(t, err)
	// Non-hex identifier.
	_, err = ParseLine("ZZZ1122", false)
	assert.Error(t, err)
	// Too short for a 29-bit header.
	_, err = ParseLine("18DA", true)
	assert.Error(t, err)
}

func TestIsStatusToken(t *testing.T) {
	assert.True(t, IsStatusToken("NO DATA"))
	assert.True(t, IsStatusToken("searching..."))
	assert.True(t, IsStatusToken("OBDLINK MX"))
	assert.False(t, IsStatusToken("7E80123"))
}
