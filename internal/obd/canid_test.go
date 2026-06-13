package obd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCANID(t *testing.T) {
	for in, want := range map[string]uint32{
		"7E8":      0x7E8,
		"0x7E8":    0x7E8,
		" 244 ":    0x244,
		"18DAF110": 0x18DAF110,
	} {
		got, err := ParseCANID(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
}

func TestParseCANID_Errors(t *testing.T) {
	_, err := ParseCANID("")
	assert.Error(t, err)
	_, err = ParseCANID("XYZ")
	assert.Error(t, err)
}

func TestParseCANIDs_SkipsBlanks(t *testing.T) {
	got, err := ParseCANIDs([]string{"7E8", "", "  ", "0x244"})
	require.NoError(t, err)
	assert.Equal(t, []uint32{0x7E8, 0x244}, got)
}
