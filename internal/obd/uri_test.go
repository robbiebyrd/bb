package obd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseURI_SerialPaths(t *testing.T) {
	for _, path := range []string{"/dev/ttyUSB0", "/dev/rfcomm0", "/dev/cu.OBDLink", "COM3"} {
		target, err := ParseURI(path)
		require.NoError(t, err, path)
		assert.Equal(t, TargetSerial, target.Kind)
		assert.Equal(t, path, target.Path)
	}
}

func TestParseURI_RFCOMMScheme(t *testing.T) {
	target, err := ParseURI("rfcomm://AA:BB:CC:DD:EE:FF")
	require.NoError(t, err)
	assert.Equal(t, TargetRFCOMM, target.Kind)
	assert.Equal(t, [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}, target.MAC)
	assert.Equal(t, "AA:BB:CC:DD:EE:FF", target.MACString)
	assert.Equal(t, uint8(0), target.Channel) // auto-discover
}

func TestParseURI_RFCOMMWithChannel(t *testing.T) {
	target, err := ParseURI("rfcomm://AA:BB:CC:DD:EE:FF/3")
	require.NoError(t, err)
	assert.Equal(t, TargetRFCOMM, target.Kind)
	assert.Equal(t, uint8(3), target.Channel)
}

func TestParseURI_BareMAC(t *testing.T) {
	target, err := ParseURI("aa:bb:cc:dd:ee:ff")
	require.NoError(t, err)
	assert.Equal(t, TargetRFCOMM, target.Kind)
	assert.Equal(t, [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}, target.MAC)
	assert.Equal(t, "AA:BB:CC:DD:EE:FF", target.MACString)
}

func TestParseURI_HyphenMAC(t *testing.T) {
	target, err := ParseURI("AA-BB-CC-DD-EE-FF")
	require.NoError(t, err)
	assert.Equal(t, TargetRFCOMM, target.Kind)
	assert.Equal(t, "AA:BB:CC:DD:EE:FF", target.MACString)
}

func TestParseURI_Errors(t *testing.T) {
	_, err := ParseURI("")
	assert.Error(t, err)
	_, err = ParseURI("rfcomm://not-a-mac")
	assert.Error(t, err)
	_, err = ParseURI("rfcomm://AA:BB:CC:DD:EE:FF/99")
	assert.Error(t, err)
}

func TestBdaddr_Reverses(t *testing.T) {
	assert.Equal(t,
		[6]byte{0xFF, 0xEE, 0xDD, 0xCC, 0xBB, 0xAA},
		bdaddr([6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}),
	)
}
