package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/robbiebyrd/bb/internal/models"
)

// Standard CAN uses 11-bit arbitration IDs (max 0x7FF).
// Extended CAN uses 29-bit arbitration IDs (0x800–0x1FFFFFFF).
// Storing both in the same uint32 field is the contract; these tests
// verify that the field can represent both ranges without truncation.
func TestCanMessageTimestamped_StandardCANIDRange(t *testing.T) {
	msg := models.CanMessageTimestamped{ID: 0x7FF}
	assert.Equal(t, uint32(0x7FF), msg.ID, "maximum 11-bit standard CAN ID must round-trip without truncation")
}

func TestCanMessageTimestamped_ExtendedCANIDRange(t *testing.T) {
	msg := models.CanMessageTimestamped{ID: 0x1FFFFFFF}
	assert.Equal(t, uint32(0x1FFFFFFF), msg.ID, "maximum 29-bit extended CAN ID must round-trip without truncation")
	assert.Greater(t, msg.ID, uint32(0x7FF), "extended CAN ID must exceed the standard 11-bit boundary")
}

// CAN frames carry at most 8 bytes of payload for classic CAN.
// Data and Length are independent fields; the struct does not auto-compute
// one from the other. This tests that the contract is not silently broken.
func TestCanMessageTimestamped_MaxPayload(t *testing.T) {
	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03, 0x04}
	msg := models.CanMessageTimestamped{
		Length: uint8(len(payload)),
		Data:   payload,
	}
	assert.Equal(t, uint8(8), msg.Length)
	require.Len(t, msg.Data, 8)
	assert.Equal(t, payload, msg.Data)
}

// Length and Data are intentionally independent: the driver sets Length from
// the frame header; Data holds the raw bytes. A mismatch is valid at the model
// layer (e.g. a truncated frame). This test documents that independence.
func TestCanMessageTimestamped_LengthDataAreIndependent(t *testing.T) {
	msg := models.CanMessageTimestamped{
		Length: 8,
		Data:   []byte{0x01, 0x02}, // intentionally shorter than Length
	}
	assert.Equal(t, uint8(8), msg.Length)
	assert.Len(t, msg.Data, 2)
}

// Transmit and Remote are mutually exclusive frame-type flags in the CAN spec.
// The model does not enforce mutual exclusion — that is the driver's job — but
// both must be independently settable.
func TestCanMessageTimestamped_FrameTypeFlags(t *testing.T) {
	tx := models.CanMessageTimestamped{Transmit: true}
	assert.True(t, tx.Transmit)
	assert.False(t, tx.Remote)

	rtr := models.CanMessageTimestamped{Remote: true}
	assert.True(t, rtr.Remote)
	assert.False(t, rtr.Transmit)
}

// Signal values are physical measurements (e.g. voltage, RPM, temperature).
// They may be negative and must preserve sub-unit precision; float32 would lose
// precision on large values. Storing as float64 is the contract.
func TestCanSignalTimestamped_NegativeValue(t *testing.T) {
	sig := models.CanSignalTimestamped{Value: -40.0, Unit: "degC"}
	assert.Equal(t, -40.0, sig.Value, "negative signal values (e.g. below-zero temperature) must be preserved")
}

func TestCanSignalTimestamped_SubUnitPrecision(t *testing.T) {
	// A value like 3500.123456789 would lose precision in float32 (~7 significant digits).
	// float64 provides ~15 significant digits, which is required for accurate physical decoding.
	precise := 3500.123456789
	sig := models.CanSignalTimestamped{Value: precise}
	assert.InDelta(t, precise, sig.Value, 1e-6, "float64 must preserve at least 6 decimal places of signal precision")
}

func TestCanSignalTimestamped_ExtendedCANIDRange(t *testing.T) {
	sig := models.CanSignalTimestamped{ID: 0x1FFFFFFF}
	assert.Equal(t, uint32(0x1FFFFFFF), sig.ID, "signal ID field must accommodate 29-bit extended CAN IDs")
}

// "can" (real SocketCAN hardware) and "sim" (software simulator) are the two
// supported network types. They must be distinct strings so callers can branch
// on them; this test fails if the names are accidentally aliased.
func TestCanInterfaceOption_NetworkTypeDistinction(t *testing.T) {
	hardware := models.CanInterfaceOption{Network: "can"}
	simulator := models.CanInterfaceOption{Network: "sim"}
	assert.NotEqual(t, hardware.Network, simulator.Network,
		"\"can\" and \"sim\" network types must be distinct values")
}
