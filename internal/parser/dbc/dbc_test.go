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
	client, err := NewDBCParserClient(l, "example.dbc")
	require.NoError(t, err)

	signals := client.ParseSignals(canModels.CanMessageData{ID: 99999, Length: 8, Data: make([]byte, 8)}, 0, 0)
	if signals != nil {
		t.Fatalf("expected nil for unknown message, got %v", signals)
	}
}

func TestNewDBCParserClient_MissingFile_ReturnsError(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	_, err := NewDBCParserClient(l, "nonexistent.dbc")
	assert.Error(t, err, "NewDBCParserClient must return an error when the DBC file cannot be loaded")
}

