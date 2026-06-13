package obd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildServiceSearchAttributeRequest(t *testing.T) {
	pdu := buildServiceSearchAttributeRequest(1)

	// PDU ID, transaction ID, parameter length header.
	assert.Equal(t, byte(sdpServiceSearchAttrRequest), pdu[0])
	assert.Equal(t, []byte{0x00, 0x01}, pdu[1:3])
	// Parameters: search pattern (SerialPort UUID) + max bytes + attr list + cont.
	expectedParams := []byte{
		0x35, 0x03, 0x19, 0x11, 0x01, // DES{ UUID16(0x1101) }
		0xFF, 0xFF, // MaximumAttributeByteCount
		0x35, 0x03, 0x09, 0x00, 0x04, // DES{ uint16(0x0004) }
		0x00, // continuation state
	}
	assert.Equal(t, []byte{0x00, byte(len(expectedParams))}, pdu[3:5])
	assert.Equal(t, expectedParams, pdu[5:])
}

func TestParseRFCOMMChannel(t *testing.T) {
	// Response body embeds an RFCOMM protocol descriptor: UUID16(0x0003) Uint8(5).
	resp := []byte{
		sdpServiceSearchAttrResponse, 0x00, 0x01, 0x00, 0x08, // header
		0x19, 0x00, 0x03, 0x08, 0x05, // RFCOMM UUID + channel 5
		0x00, 0x00, 0x00,
	}
	ch, err := parseRFCOMMChannel(resp)
	require.NoError(t, err)
	assert.Equal(t, uint8(5), ch)
}

func TestParseRFCOMMChannel_Errors(t *testing.T) {
	// Wrong PDU id.
	_, err := parseRFCOMMChannel([]byte{0x06, 0, 1, 0, 0})
	assert.Error(t, err)
	// No RFCOMM descriptor present.
	_, err = parseRFCOMMChannel([]byte{sdpServiceSearchAttrResponse, 0, 1, 0, 3, 0x19, 0x01, 0x00})
	assert.Error(t, err)
}
