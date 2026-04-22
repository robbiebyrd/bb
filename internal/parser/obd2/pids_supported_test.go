package obd2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	canModels "github.com/robbiebyrd/cantou/internal/models"
)

func TestDecodePIDsSupported_WikipediaExample(t *testing.T) {
	// Wikipedia example: BE1FA813 for PID range $01-$20
	// Supported: 01,03,04,05,06,07,0C,0D,0E,0F,10,11,13,15,1C,1F,20
	got := DecodePIDsSupported(0xBE1FA813, 0x00)
	want := []uint8{
		0x01, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x0C, 0x0D, 0x0E, 0x0F, 0x10,
		0x11, 0x13, 0x15,
		0x1C, 0x1F, 0x20,
	}
	assert.Equal(t, want, got)
}

func TestDecodePIDsSupported_AllSet(t *testing.T) {
	got := DecodePIDsSupported(0xFFFFFFFF, 0x00)
	assert.Len(t, got, 32)
	assert.Equal(t, uint8(0x01), got[0])
	assert.Equal(t, uint8(0x20), got[31])
}

func TestDecodePIDsSupported_NoneSet(t *testing.T) {
	got := DecodePIDsSupported(0x00000000, 0x00)
	assert.Empty(t, got)
}

func TestDecodePIDsSupported_OnlyFirstBit(t *testing.T) {
	got := DecodePIDsSupported(0x80000000, 0x00)
	assert.Equal(t, []uint8{0x01}, got)
}

func TestDecodePIDsSupported_OnlyLastBit(t *testing.T) {
	got := DecodePIDsSupported(0x00000001, 0x00)
	assert.Equal(t, []uint8{0x20}, got)
}

func TestDecodePIDsSupported_Range21to40(t *testing.T) {
	got := DecodePIDsSupported(0x80000000, 0x20)
	assert.Equal(t, []uint8{0x21}, got)
}

func TestDecodePIDsSupported_Range41to60(t *testing.T) {
	got := DecodePIDsSupported(0x00000001, 0x40)
	assert.Equal(t, []uint8{0x60}, got)
}

func TestDecodePIDsSupported_MercedesE350_PID00(t *testing.T) {
	// Real data from Mercedes E350 2010: S01PID00_PIDsSupported_01_20 = 2554044435
	// 2554044435 = 0x983BA013
	got := DecodePIDsSupported(2554044435, 0x00)
	want := []uint8{0x01, 0x04, 0x05, 0x0B, 0x0C, 0x0D, 0x0F, 0x10, 0x11, 0x13, 0x1C, 0x1F, 0x20}
	assert.Equal(t, want, got)
}

func TestDecodePIDsSupported_MercedesE350_PID20(t *testing.T) {
	// Real data: S01PID20_PIDsSupported_21_40 = 2953027589
	// 2953027589 = 0xB003A005
	got := DecodePIDsSupported(2953027589, 0x20)
	want := []uint8{0x21, 0x23, 0x24, 0x2F, 0x30, 0x31, 0x33, 0x3E, 0x40}
	assert.Equal(t, want, got)
}

func TestDecodePIDsSupported_MercedesE350_PID40(t *testing.T) {
	// Real data: S01PID40_PIDsSupported_41_60 = 1289224193
	// 1289224193 = 0x4CD80001
	got := DecodePIDsSupported(1289224193, 0x40)
	want := []uint8{0x42, 0x45, 0x46, 0x49, 0x4A, 0x4C, 0x4D, 0x60}
	assert.Equal(t, want, got)
}

func TestDecodePIDsSupported_MercedesE350_PID60(t *testing.T) {
	// Real data: S01PID60_PIDsSupported_61_80 = 84410368
	// 84410368 = 0x05080000
	got := DecodePIDsSupported(84410368, 0x60)
	want := []uint8{0x66, 0x68, 0x6D}
	assert.Equal(t, want, got)
}

func TestExpandPIDsSupported_NonPIDsSignal_Passthrough(t *testing.T) {
	sig := canModels.CanSignalTimestamped{Signal: "EngineRPM", Value: 3000}
	got := ExpandPIDsSupported(sig)
	assert.Equal(t, []canModels.CanSignalTimestamped{sig}, got)
}

func TestExpandPIDsSupported_PIDsSignal_Expanded(t *testing.T) {
	// 0x80000001 => bit 31 (PID 0x01) and bit 0 (PID 0x20) are set
	sig := canModels.CanSignalTimestamped{
		Timestamp: 1000,
		Interface: 2,
		ID:        0x7DF,
		Message:   "OBD2_Mode01",
		Signal:    "S01PID00_PIDsSupported_01_20",
		Value:     0x80000001,
		Unit:      "",
	}
	got := ExpandPIDsSupported(sig)
	assert.Len(t, got, 2)
	var names []string
	for _, s := range got {
		names = append(names, s.Signal)
		assert.Equal(t, float64(1), s.Value)
		assert.Equal(t, int64(1000), s.Timestamp)
		assert.Equal(t, 2, s.Interface)
		assert.Equal(t, "OBD2_Mode01", s.Message)
	}
	assert.Contains(t, names, "S01PID_Supported_01")
	assert.Contains(t, names, "S01PID_Supported_20")
}

func TestExpandPIDsSupported_EmptyBitmask_ReturnsEmpty(t *testing.T) {
	sig := canModels.CanSignalTimestamped{
		Signal: "S01PID00_PIDsSupported_01_20",
		Value:  0,
	}
	got := ExpandPIDsSupported(sig)
	assert.Empty(t, got)
}

func TestIsPIDsSupportedSignal(t *testing.T) {
	tests := []struct {
		name     string
		signal   string
		wantOk   bool
		wantBase uint8
	}{
		{"PID00", "S01PID00_PIDsSupported_01_20", true, 0x00},
		{"PID20", "S01PID20_PIDsSupported_21_40", true, 0x20},
		{"PID40", "S01PID40_PIDsSupported_41_60", true, 0x40},
		{"PID60", "S01PID60_PIDsSupported_61_80", true, 0x60},
		{"PID80", "S01PID80_PIDsSupported_81_A0", true, 0x80},
		{"PIDA0", "S01PIDA0_PIDsSupported_A1_C0", true, 0xA0},
		{"PIDC0", "S01PIDC0_PIDsSupported_C1_E0", true, 0xC0},
		{"not supported signal", "S01PID0C_EngineRPM", false, 0},
		{"empty", "", false, 0},
		{"mux signal", "S01PID", false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ok := IsPIDsSupportedSignal(tt.signal)
			assert.Equal(t, tt.wantOk, ok)
			if ok {
				assert.Equal(t, tt.wantBase, base)
			}
		})
	}
}
