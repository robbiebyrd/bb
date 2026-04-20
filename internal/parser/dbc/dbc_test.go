package dbc

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

func TestLoad(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client, err := NewDBCParserClient(l, "example.dbc")
	require.NoError(t, err)
	dc := client.(*DBCParserClient)

	if dc.db == nil {
		t.Fatal("expected database to be loaded")
	}
	if len(dc.db.Messages) == 0 {
		t.Fatal("expected messages to be loaded")
	}
	// The example DBC has message TORQ_0A8 at ID 168.
	msg, ok := dc.db.Message(168)
	if !ok {
		t.Fatal("expected message ID 168 (TORQ_0A8)")
	}
	if msg.Name != "TORQ_0A8" {
		t.Fatalf("expected message name TORQ_0A8, got %s", msg.Name)
	}
	if len(msg.Signals) != 5 {
		t.Fatalf("expected 5 signals in TORQ_0A8, got %d", len(msg.Signals))
	}
}

func TestLoad_FileNotFound_ReturnsError(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	_, err := NewDBCParserClient(l, "nonexistent.dbc")
	assert.Error(t, err)
}

func TestParseSignals_KnownMessage(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client, err := NewDBCParserClient(l, "example.dbc")
	require.NoError(t, err)

	// MOTOR_1D0 (ID=464): EngineCoolantTemp = scale 1, offset -48, unit "degC"
	// byte 0 = 88 → 88 - 48 = 40 degC
	data := make([]byte, 8)
	data[0] = 88
	msg := canModels.CanMessageTimestamped{ID: 464, Length: 8, Data: data, Timestamp: 1_000_000_000, Interface: 3}

	signals := client.ParseSignals(msg)

	if len(signals) == 0 {
		t.Fatal("expected signals, got none")
	}

	var coolant *canModels.CanSignalTimestamped
	for i := range signals {
		if signals[i].Signal == "EngineCoolantTemp" {
			coolant = &signals[i]
		}
	}
	if coolant == nil {
		t.Fatal("expected EngineCoolantTemp signal")
	}
	if coolant.Timestamp != 1_000_000_000 {
		t.Errorf("expected timestamp 1000000000, got %d", coolant.Timestamp)
	}
	if coolant.Interface != 3 {
		t.Errorf("expected interface 3, got %d", coolant.Interface)
	}
	if coolant.ID != 464 {
		t.Errorf("expected ID 464, got %d", coolant.ID)
	}
	if coolant.Message != "MOTOR_1D0" {
		t.Errorf("expected message MOTOR_1D0, got %q", coolant.Message)
	}
	if coolant.Unit != "degC" {
		t.Errorf("expected unit degC, got %q", coolant.Unit)
	}
	if coolant.Value != 40.0 {
		t.Errorf("expected value 40.0, got %v", coolant.Value)
	}
}

func TestParseSignals_UnitsPopulated(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client, err := NewDBCParserClient(l, "example.dbc")
	require.NoError(t, err)

	// STEER_0C4 (ID=196): SteeringPosition unit="deg", SteeringSpeed unit="deg/s"
	msg := canModels.CanMessageTimestamped{ID: 196, Length: 7, Data: make([]byte, 7)}
	signals := client.ParseSignals(msg)

	units := make(map[string]string)
	for _, s := range signals {
		units[s.Signal] = s.Unit
	}
	if units["SteeringPosition"] != "deg" {
		t.Errorf("expected SteeringPosition unit deg, got %q", units["SteeringPosition"])
	}
	if units["SteeringSpeed"] != "deg/s" {
		t.Errorf("expected SteeringSpeed unit deg/s, got %q", units["SteeringSpeed"])
	}
}

func TestParseSignals_UnknownMessage(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client, err := NewDBCParserClient(l, "example.dbc")
	require.NoError(t, err)

	signals := client.ParseSignals(canModels.CanMessageTimestamped{ID: 99999, Length: 8, Data: make([]byte, 8)})
	if signals != nil {
		t.Fatalf("expected nil for unknown message, got %v", signals)
	}
}

func TestNewDBCParserClient_MissingFile_ReturnsError(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	_, err := NewDBCParserClient(l, "nonexistent.dbc")
	assert.Error(t, err, "NewDBCParserClient must return an error when the DBC file cannot be loaded")
}

// muxClient loads mux_test.dbc, which has one multiplexed message:
//
//	BO_ 2024 OBD2_ECU0: 8
//	  SG_ PID M     : 16|8@1+  (1,0)     — mux signal (byte 2)
//	  SG_ CoolantTemp m5  : 24|8@1+  (1,-40)  — active when PID=5
//	  SG_ EngineRPM  m12 : 31|16@0+ (0.25,0) — active when PID=12
//	  SG_ VehicleSpeed m13 : 24|8@1+ (1,0)   — active when PID=13
func muxClient(t *testing.T) canModels.ParserInterface {
	t.Helper()
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client, err := NewDBCParserClient(l, "mux_test.dbc")
	require.NoError(t, err)
	return client
}

func signalMap(sigs []canModels.CanSignalTimestamped) map[string]float64 {
	m := make(map[string]float64, len(sigs))
	for _, s := range sigs {
		m[s.Signal] = s.Value
	}
	return m
}

func TestParseSignals_Multiplexed_OnlyActiveSignalDecoded(t *testing.T) {
	client := muxClient(t)

	// PID=5 (CoolantTemp): byte2=0x05, byte3=88 → 88-40=48 degC
	data := []byte{0x04, 0x41, 0x05, 88, 0x00, 0x00, 0x00, 0x00}
	msg := canModels.CanMessageTimestamped{ID: 2024, Length: 8, Data: data, Timestamp: 1_000_000_000, Interface: 0}

	signals := client.ParseSignals(msg)
	got := signalMap(signals)

	assert.InDelta(t, 48.0, got["CoolantTemp"], 0.001)
	assert.NotContains(t, got, "EngineRPM", "EngineRPM must not be decoded when PID=5")
	assert.NotContains(t, got, "VehicleSpeed", "VehicleSpeed must not be decoded when PID=5")
}

func TestParseSignals_Multiplexed_RPMDecoded(t *testing.T) {
	client := muxClient(t)

	// PID=12 (EngineRPM): byte2=0x0C, byte3=0x1A=26, byte4=0x28=40
	// (26*256+40)*0.25 = 6696*0.25 = 1674.0 RPM
	data := []byte{0x04, 0x41, 0x0C, 0x1A, 0x28, 0x00, 0x00, 0x00}
	msg := canModels.CanMessageTimestamped{ID: 2024, Length: 8, Data: data, Timestamp: 1_000_000_000, Interface: 0}

	signals := client.ParseSignals(msg)
	got := signalMap(signals)

	assert.InDelta(t, 1674.0, got["EngineRPM"], 0.001)
	assert.NotContains(t, got, "CoolantTemp", "CoolantTemp must not be decoded when PID=12")
	assert.NotContains(t, got, "VehicleSpeed", "VehicleSpeed must not be decoded when PID=12")
}

func TestParseSignals_Multiplexed_UnknownMuxValueEmitsNoMultiplexedSignals(t *testing.T) {
	client := muxClient(t)

	// PID=99 — no signal defined for this mux value
	data := []byte{0x04, 0x41, 0x63, 0x00, 0x00, 0x00, 0x00, 0x00}
	msg := canModels.CanMessageTimestamped{ID: 2024, Length: 8, Data: data}

	signals := client.ParseSignals(msg)
	got := signalMap(signals)

	assert.NotContains(t, got, "CoolantTemp")
	assert.NotContains(t, got, "EngineRPM")
	assert.NotContains(t, got, "VehicleSpeed")
	// The mux signal itself (PID) is always present
	assert.Contains(t, got, "PID")
}

// obd2Client loads the standard OBD-II DBC (nested two-level mux: S → S01PID → signals).
func obd2Client(t *testing.T) canModels.ParserInterface {
	t.Helper()
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client, err := NewDBCParserClient(l, "../../../dbcs/obd2.dbc")
	require.NoError(t, err)
	return client
}

// TestParseSignals_NestedMux_EngineRPM verifies that two-level mux decoding works:
// S=0x41 (Mode 01) selects S01PID as nested mux; S01PID=0x0C selects EngineRPM.
func TestParseSignals_NestedMux_EngineRPM(t *testing.T) {
	client := obd2Client(t)

	// S=0x41, PID=0x0C, A=0x1A=26, B=0x28=40 → (26*256+40)/4 = 1674.0 rpm
	data := []byte{0x04, 0x41, 0x0C, 0x1A, 0x28, 0x00, 0x00, 0x00}
	msg := canModels.CanMessageTimestamped{ID: 2024, Length: 8, Data: data}

	got := signalMap(client.ParseSignals(msg))

	assert.Contains(t, got, "S01PID0C_EngineRPM")
	assert.InDelta(t, 1674.0, got["S01PID0C_EngineRPM"], 0.001)
	assert.NotContains(t, got, "S01PID0D_VehicleSpeed", "wrong-PID signals must be excluded")
	assert.NotContains(t, got, "S01PID05_EngineCoolantTemp", "wrong-PID signals must be excluded")
}

// TestParseSignals_NestedMux_EngineCoolantTemp verifies a different PID value in the nested mux.
func TestParseSignals_NestedMux_EngineCoolantTemp(t *testing.T) {
	client := obd2Client(t)

	// S=0x41, PID=0x05, A=88 → 88-40 = 48 degC
	data := []byte{0x04, 0x41, 0x05, 88, 0x00, 0x00, 0x00, 0x00}
	msg := canModels.CanMessageTimestamped{ID: 2024, Length: 8, Data: data}

	got := signalMap(client.ParseSignals(msg))

	assert.Contains(t, got, "S01PID05_EngineCoolantTemp")
	assert.InDelta(t, 48.0, got["S01PID05_EngineCoolantTemp"], 0.001)
	assert.NotContains(t, got, "S01PID0C_EngineRPM")
}

// TestParseSignals_NestedMux_FlatMuxUnaffected confirms that existing flat-mux behaviour
// (mux_test.dbc) is unaffected by the nested mux changes.
func TestParseSignals_NestedMux_FlatMuxUnaffected(t *testing.T) {
	client := muxClient(t)

	data := []byte{0x04, 0x41, 0x05, 88, 0x00, 0x00, 0x00, 0x00}
	msg := canModels.CanMessageTimestamped{ID: 2024, Length: 8, Data: data}

	got := signalMap(client.ParseSignals(msg))

	assert.Contains(t, got, "CoolantTemp")
	assert.InDelta(t, 48.0, got["CoolantTemp"], 0.001)
}

// TestNewDBCParserClientFromBytes verifies that a parser can be constructed from raw bytes.
func TestNewDBCParserClientFromBytes(t *testing.T) {
	data, err := os.ReadFile("../../../dbcs/obd2.dbc")
	require.NoError(t, err)

	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client, err := NewDBCParserClientFromBytes(l, "obd2-test", data)
	require.NoError(t, err)

	// Sanity-check: PID 0x0C should still decode correctly.
	frame := []byte{0x04, 0x41, 0x0C, 0x1A, 0x28, 0x00, 0x00, 0x00}
	msg := canModels.CanMessageTimestamped{ID: 2024, Length: 8, Data: frame}
	got := signalMap(client.ParseSignals(msg))
	assert.InDelta(t, 1674.0, got["S01PID0C_EngineRPM"], 0.001)
}

// TestMultiParser_CombinesSignals verifies that MultiParser merges results from all parsers.
// mux_test.dbc and obd2.dbc both contain message ID 2024; with the same frame the
// multi-parser should return signals from both.
func TestMultiParser_CombinesSignals(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))

	p1, err := NewDBCParserClient(l, "mux_test.dbc")
	require.NoError(t, err)
	p2, err := NewDBCParserClient(l, "../../../dbcs/obd2.dbc")
	require.NoError(t, err)

	multi := NewMultiParser(p1, p2)

	// PID=5: p1 (mux_test.dbc) → "CoolantTemp"; p2 (obd2.dbc) → "S01PID05_EngineCoolantTemp"
	data := []byte{0x04, 0x41, 0x05, 88, 0x00, 0x00, 0x00, 0x00}
	msg := canModels.CanMessageTimestamped{ID: 2024, Length: 8, Data: data}

	got := signalMap(multi.ParseSignals(msg))

	assert.Contains(t, got, "CoolantTemp", "p1 (mux_test.dbc) signal must be present")
	assert.Contains(t, got, "S01PID05_EngineCoolantTemp", "p2 (obd2.dbc) signal must be present")
}

