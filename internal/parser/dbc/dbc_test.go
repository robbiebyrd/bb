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
	client := NewDBCParserClient(l, 0, "example.dbc")
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
	client := NewDBCParserClient(l, 0, "nonexistent.dbc")
	dc := client.(*DBCParserClient)

	if dc.db != nil {
		t.Fatal("expected database to be nil for missing file")
	}
}

func TestParse_KnownMessage(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client := NewDBCParserClient(l, 0, "example.dbc")

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
	client := NewDBCParserClient(l, 0, "example.dbc")

	result := client.Parse(canModels.CanMessageData{
		ID:     99999,
		Length: 8,
		Data:   []byte{0, 0, 0, 0, 0, 0, 0, 0},
	})
	if result != nil {
		t.Fatalf("expected nil for unknown message, got %v", result)
	}
}

func TestParse_NilDatabase(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	client := NewDBCParserClient(l, 0, "nonexistent.dbc")

	result := client.Parse(canModels.CanMessageData{
		ID:     168,
		Length: 8,
		Data:   []byte{0, 0, 0, 0, 0, 0, 0, 0},
	})
	if result != nil {
		t.Fatalf("expected nil for nil database, got %v", result)
	}
}
