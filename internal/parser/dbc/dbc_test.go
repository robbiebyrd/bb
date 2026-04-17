package dbc

import (
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

func TestLoad(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client := NewDBCParserClient(l,"example.dbc")
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

func TestLoad_FileNotFound(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client := NewDBCParserClient(l,"nonexistent.dbc")
	dc := client.(*DBCParserClient)

	if dc.db != nil {
		t.Fatal("expected database to be nil for missing file")
	}
}

func TestParse_KnownMessage(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client := NewDBCParserClient(l,"example.dbc")

	// Message ID 168 = TORQ_0A8 (8 bytes, from DME).
	// Signals:
	//   ClutchStatus:    bit 0, 1 bit, LE, unsigned => data[0] & 0x01
	//   EngineTorque:    bits 8-23, 16 bits, LE, signed
	//   EngineTorqueRaw: bits 24-39, 16 bits, LE, signed
	//   BrakeStatus:     bit 61, 1 bit, BE, unsigned (byte 7, bit 5)
	//   BrakeStatus2:    bit 62, 1 bit, BE, unsigned (byte 7, bit 6)
	//
	// Set ClutchStatus=1 (Pressed), EngineTorque=100, BrakeStatus=1 (Pressed).
	data := []byte{
		0x01,       // byte 0: ClutchStatus=1
		100, 0x00,  // bytes 1-2: EngineTorque=100 (little-endian at bit 8)
		0x00, 0x00, // bytes 3-4: EngineTorqueRaw=0
		0x00, 0x00, // bytes 5-6
		0x20, // byte 7: bit 5 set => BrakeStatus=1
	}

	result := client.Parse(canModels.CanMessageData{
		ID:     168,
		Length: 8,
		Data:   data,
	})

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	jsonStr, ok := result.(string)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}

	var parsed parseResult
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if parsed.Message != "TORQ_0A8" {
		t.Fatalf("expected message TORQ_0A8, got %s", parsed.Message)
	}

	// ClutchStatus should be "Pressed" (value description for 1).
	if v, ok := parsed.Signals["ClutchStatus"]; !ok {
		t.Fatal("expected ClutchStatus signal")
	} else if v != "Pressed" {
		t.Fatalf("expected ClutchStatus=Pressed, got %v", v)
	}

	// BrakeStatus should be "Pressed" (value description for 1).
	if v, ok := parsed.Signals["BrakeStatus"]; !ok {
		t.Fatal("expected BrakeStatus signal")
	} else if v != "Pressed" {
		t.Fatalf("expected BrakeStatus=Pressed, got %v", v)
	}

	// EngineTorque should be 100 (numeric).
	if v, ok := parsed.Signals["EngineTorque"]; !ok {
		t.Fatal("expected EngineTorque signal")
	} else {
		fv, ok := v.(float64)
		if !ok {
			t.Fatalf("expected float64 for EngineTorque, got %T", v)
		}
		if fv != 100 {
			t.Fatalf("expected EngineTorque=100, got %v", fv)
		}
	}
}

func TestParse_UnknownMessage(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client := NewDBCParserClient(l,"example.dbc")

	result := client.Parse(canModels.CanMessageData{
		ID:     99999,
		Length: 8,
		Data:   []byte{0, 0, 0, 0, 0, 0, 0, 0},
	})
	if result != nil {
		t.Fatalf("expected nil for unknown message, got %v", result)
	}
}

func TestParseSignals_KnownMessage(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client := NewDBCParserClient(l, "example.dbc")

	// MOTOR_1D0 (ID=464): EngineCoolantTemp = scale 1, offset -48, unit "degC"
	// byte 0 = 88 → 88 - 48 = 40 degC
	data := make([]byte, 8)
	data[0] = 88
	msg := canModels.CanMessageData{ID: 464, Length: 8, Data: data}

	signals := client.ParseSignals(msg, 1_000_000_000, 3)

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
	if coolant.Unit != "degC" {
		t.Errorf("expected unit degC, got %q", coolant.Unit)
	}
	if coolant.Value != 40.0 {
		t.Errorf("expected value 40.0, got %v", coolant.Value)
	}
}

func TestParseSignals_UnitsPopulated(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client := NewDBCParserClient(l, "example.dbc")

	// STEER_0C4 (ID=196): SteeringPosition unit="deg", SteeringSpeed unit="deg/s"
	msg := canModels.CanMessageData{ID: 196, Length: 7, Data: make([]byte, 7)}
	signals := client.ParseSignals(msg, 0, 0)

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
	client := NewDBCParserClient(l, "example.dbc")

	signals := client.ParseSignals(canModels.CanMessageData{ID: 99999, Length: 8, Data: make([]byte, 8)}, 0, 0)
	if signals != nil {
		t.Fatalf("expected nil for unknown message, got %v", signals)
	}
}

func TestParseSignals_NilDatabase(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client := NewDBCParserClient(l, "nonexistent.dbc")

	signals := client.ParseSignals(canModels.CanMessageData{ID: 464, Length: 8, Data: make([]byte, 8)}, 0, 0)
	if signals != nil {
		t.Fatalf("expected nil for nil database, got %v", signals)
	}
}

func TestParse_NilDatabase(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client := NewDBCParserClient(l,"nonexistent.dbc")

	result := client.Parse(canModels.CanMessageData{
		ID:     168,
		Length: 8,
		Data:   []byte{0, 0, 0, 0, 0, 0, 0, 0},
	})
	if result != nil {
		t.Fatalf("expected nil for nil database, got %v", result)
	}
}
