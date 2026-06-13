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

func TestOBDResponseIDs_11Bit(t *testing.T) {
	ids := OBDResponseIDs(false)
	assert.Equal(t, []uint32{0x7E8, 0x7E9, 0x7EA, 0x7EB, 0x7EC, 0x7ED, 0x7EE, 0x7EF}, ids)
}

func TestOBDResponseIDs_29Bit(t *testing.T) {
	ids := OBDResponseIDs(true)
	assert.Len(t, ids, 16)
	assert.Equal(t, uint32(0x18DAF100), ids[0])
	assert.Equal(t, uint32(0x18DAF10F), ids[15])
}

func TestMergeUniqueIDs(t *testing.T) {
	got := MergeUniqueIDs([]uint32{0x1C4, 0x7E8}, []uint32{0x7E8, 0x7E9})
	assert.Equal(t, []uint32{0x1C4, 0x7E8, 0x7E9}, got)
}
