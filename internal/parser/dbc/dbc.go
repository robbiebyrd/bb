package dbc

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"go.einride.tech/can"
	"go.einride.tech/can/pkg/dbc"
	"go.einride.tech/can/pkg/descriptor"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

type DBCParserClient struct {
	l           *slog.Logger
	dbcFilePath string
	db          *descriptor.Database
}

func NewDBCParserClient(
	l *slog.Logger,
	dbcFilePath string,
) (canModels.ParserInterface, error) {
	dc := &DBCParserClient{l: l, dbcFilePath: dbcFilePath}
	if err := dc.Load(); err != nil {
		return nil, fmt.Errorf("loading DBC file %s: %w", dbcFilePath, err)
	}
	return dc, nil
}

// Load reads and parses the DBC file into an in-memory descriptor.Database.
func (dc *DBCParserClient) Load() error {
	data, err := os.ReadFile(dc.dbcFilePath)
	if err != nil {
		return fmt.Errorf("reading DBC file: %w", err)
	}
	p := dbc.NewParser(dc.dbcFilePath, data)
	if parseErr := p.Parse(); parseErr != nil {
		return fmt.Errorf("parsing DBC file: %w", parseErr)
	}
	dc.db = compile(dc.dbcFilePath, p.Defs())
	dc.l.Info("loaded DBC file", "path", dc.dbcFilePath, "messages", len(dc.db.Messages))
	return nil
}

// ParseSignals decodes all signals in msg and returns one CanSignalTimestamped
// per decoded signal. Returns nil if the message ID is not in the database.
//
// Supports single-level (flat) and two-level (nested) multiplexed messages.
// For nested mux, a signal marked mNM in the DBC (IsMultiplexer && IsMultiplexed)
// is only active when the top mux value equals N; leaf signals under it are then
// filtered by that nested mux's current value. The mux switch signals themselves
// are always included.
func (dc *DBCParserClient) ParseSignals(msg canModels.CanMessageTimestamped) []canModels.CanSignalTimestamped {
	if dc.db == nil {
		return nil
	}
	message, ok := dc.db.Message(msg.ID)
	if !ok {
		return nil
	}
	var canData can.Data
	copy(canData[:], msg.Data)

	// Find the top-level mux switch (IsMultiplexer=true, IsMultiplexed=false).
	var topMux *descriptor.Signal
	var topMuxValue uint64
	for _, sig := range message.Signals {
		if sig.IsMultiplexer && !sig.IsMultiplexed {
			topMux = sig
			topMuxValue = sig.UnmarshalUnsigned(canData)
			break
		}
	}

	// Find the active nested mux switch (IsMultiplexer && IsMultiplexed && value==topMuxValue).
	var nestedMux *descriptor.Signal
	var nestedMuxValue uint64
	if topMux != nil {
		for _, sig := range message.Signals {
			if sig.IsMultiplexer && sig.IsMultiplexed && uint64(sig.MultiplexerValue) == topMuxValue {
				nestedMux = sig
				nestedMuxValue = sig.UnmarshalUnsigned(canData)
				break
			}
		}
	}

	signals := make([]canModels.CanSignalTimestamped, 0, len(message.Signals))
	for _, sig := range message.Signals {
		switch {
		case !sig.IsMultiplexed:
			// Always include: non-multiplexed signals and the top mux switch itself.
		case sig.IsMultiplexer && sig.IsMultiplexed:
			// Nested mux switch: include only when active (parent mux value matches).
			if topMux == nil || uint64(sig.MultiplexerValue) != topMuxValue {
				continue
			}
		case nestedMux != nil:
			// Leaf under a two-level mux: filter by the nested mux's current value.
			if uint64(sig.MultiplexerValue) != nestedMuxValue {
				continue
			}
		default:
			// Flat-mux leaf: filter by the top mux's current value.
			if topMux == nil || uint64(sig.MultiplexerValue) != topMuxValue {
				continue
			}
		}
		signals = append(signals, canModels.CanSignalTimestamped{
			Timestamp: msg.Timestamp,
			Interface: msg.Interface,
			ID:        msg.ID,
			Message:   message.Name,
			Signal:    sig.Name,
			Value:     sig.UnmarshalPhysical(canData),
			Unit:      sig.Unit,
		})
	}
	return signals
}

// NewDBCParserClientFromBytes constructs a parser from raw DBC file bytes.
// name is used only for logging and error messages.
func NewDBCParserClientFromBytes(l *slog.Logger, name string, data []byte) (canModels.ParserInterface, error) {
	p := dbc.NewParser(name, data)
	if parseErr := p.Parse(); parseErr != nil {
		return nil, fmt.Errorf("parsing DBC data %s: %w", name, parseErr)
	}
	dc := &DBCParserClient{l: l, dbcFilePath: name, db: compile(name, p.Defs())}
	dc.l.Info("loaded DBC data", "name", name, "messages", len(dc.db.Messages))
	return dc, nil
}

// MultiParser combines multiple ParserInterface implementations. ParseSignals
// returns the union of signals from all parsers.
type MultiParser struct {
	parsers []canModels.ParserInterface
}

// NewMultiParser wraps one or more parsers so their results are merged on each call.
func NewMultiParser(parsers ...canModels.ParserInterface) canModels.ParserInterface {
	return &MultiParser{parsers: parsers}
}

func (m *MultiParser) ParseSignals(msg canModels.CanMessageTimestamped) []canModels.CanSignalTimestamped {
	var all []canModels.CanSignalTimestamped
	for _, p := range m.parsers {
		all = append(all, p.ParseSignals(msg)...)
	}
	return all
}

// compile converts parsed DBC definitions into a descriptor.Database.
// This replicates the logic from go.einride.tech/can/internal/generate.Compile
// which is inaccessible due to being in an internal package.
func compile(sourceFile string, defs []dbc.Def) *descriptor.Database {
	db := &descriptor.Database{SourceFile: sourceFile}

	// First pass: collect messages, signals, nodes, and version.
	for _, def := range defs {
		switch d := def.(type) {
		case *dbc.VersionDef:
			db.Version = d.Version
		case *dbc.MessageDef:
			if d.MessageID == dbc.IndependentSignalsMessageID {
				continue
			}
			msg := &descriptor.Message{
				Name:       string(d.Name),
				ID:         d.MessageID.ToCAN(),
				IsExtended: d.MessageID.IsExtended(),
				Length:     uint8(d.Size),
				SenderNode: string(d.Transmitter),
			}
			for _, sigDef := range d.Signals {
				sig := &descriptor.Signal{
					Name:             string(sigDef.Name),
					Start:            uint8(sigDef.StartBit),
					Length:           uint8(sigDef.Size),
					IsBigEndian:      sigDef.IsBigEndian,
					IsSigned:         sigDef.IsSigned,
					IsMultiplexer:    sigDef.IsMultiplexerSwitch,
					IsMultiplexed:    sigDef.IsMultiplexed,
					MultiplexerValue: uint(sigDef.MultiplexerSwitch),
					Scale:            sigDef.Factor,
					Offset:           sigDef.Offset,
					Min:              sigDef.Minimum,
					Max:              sigDef.Maximum,
					Unit:             sigDef.Unit,
				}
				for _, receiver := range sigDef.Receivers {
					sig.ReceiverNodes = append(sig.ReceiverNodes, string(receiver))
				}
				msg.Signals = append(msg.Signals, sig)
			}
			db.Messages = append(db.Messages, msg)
		case *dbc.NodesDef:
			for _, node := range d.NodeNames {
				db.Nodes = append(db.Nodes, &descriptor.Node{Name: string(node)})
			}
		}
	}

	// Second pass: apply metadata (comments, value descriptions, attributes).
	for _, def := range defs {
		switch d := def.(type) {
		case *dbc.CommentDef:
			switch d.ObjectType {
			case dbc.ObjectTypeMessage:
				if d.MessageID == dbc.IndependentSignalsMessageID {
					continue
				}
				if msg, ok := db.Message(d.MessageID.ToCAN()); ok {
					msg.Description = d.Comment
				}
			case dbc.ObjectTypeSignal:
				if d.MessageID == dbc.IndependentSignalsMessageID {
					continue
				}
				if sig, ok := db.Signal(d.MessageID.ToCAN(), string(d.SignalName)); ok {
					sig.Description = d.Comment
				}
			case dbc.ObjectTypeNetworkNode:
				if node, ok := db.Node(string(d.NodeName)); ok {
					node.Description = d.Comment
				}
			}
		case *dbc.ValueDescriptionsDef:
			if d.MessageID == dbc.IndependentSignalsMessageID {
				continue
			}
			if d.ObjectType != dbc.ObjectTypeSignal {
				continue
			}
			if sig, ok := db.Signal(d.MessageID.ToCAN(), string(d.SignalName)); ok {
				for _, vd := range d.ValueDescriptions {
					sig.ValueDescriptions = append(sig.ValueDescriptions, &descriptor.ValueDescription{
						Value:       int64(vd.Value),
						Description: vd.Description,
					})
				}
			}
		case *dbc.SignalValueTypeDef:
			if sig, ok := db.Signal(d.MessageID.ToCAN(), string(d.SignalName)); ok {
				if d.SignalValueType == dbc.SignalValueTypeFloat32 && sig.Length == 32 {
					sig.IsFloat = true
				}
			}
		case *dbc.AttributeValueForObjectDef:
			switch d.ObjectType {
			case dbc.ObjectTypeMessage:
				if msg, ok := db.Message(d.MessageID.ToCAN()); ok {
					switch d.AttributeName {
					case "GenMsgSendType":
						_ = msg.SendType.UnmarshalString(d.StringValue)
					case "GenMsgCycleTime":
						msg.CycleTime = time.Duration(d.IntValue) * time.Millisecond
					case "GenMsgDelayTime":
						msg.DelayTime = time.Duration(d.IntValue) * time.Millisecond
					}
				}
			case dbc.ObjectTypeSignal:
				if sig, ok := db.Signal(d.MessageID.ToCAN(), string(d.SignalName)); ok {
					if d.AttributeName == "GenSigStartValue" {
						sig.DefaultValue = int(d.IntValue)
					}
				}
			}
		}
	}

	// Sort for deterministic output.
	sort.Slice(db.Nodes, func(i, j int) bool {
		return db.Nodes[i].Name < db.Nodes[j].Name
	})
	sort.Slice(db.Messages, func(i, j int) bool {
		return db.Messages[i].ID < db.Messages[j].ID
	})
	for _, m := range db.Messages {
		sort.Slice(m.Signals, func(i, j int) bool {
			if m.Signals[i].MultiplexerValue < m.Signals[j].MultiplexerValue {
				return true
			}
			return m.Signals[i].Start < m.Signals[j].Start
		})
		for _, s := range m.Signals {
			sort.Slice(s.ValueDescriptions, func(i, j int) bool {
				return s.ValueDescriptions[i].Value < s.ValueDescriptions[j].Value
			})
		}
	}

	return db
}
