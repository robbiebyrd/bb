package dbc

import (
	"encoding/json"
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
	timeout int,
	dbcFilePath string,
) canModels.ParserInterface {
	dc := &DBCParserClient{l: l, dbcFilePath: dbcFilePath}
	if err := dc.Load(); err != nil {
		l.Error("failed to load DBC file", "path", dbcFilePath, "error", err)
	}
	return dc
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

type parseResult struct {
	Message string         `json:"message"`
	Signals map[string]any `json:"signals"`
}

// Parse accepts a CAN message, looks up its definition in the loaded DBC database,
// decodes all signals, and returns a JSON string.
func (dc *DBCParserClient) Parse(msg canModels.CanMessageData) any {
	if dc.db == nil {
		return nil
	}
	message, ok := dc.db.Message(msg.ID)
	if !ok {
		return nil
	}
	var canData can.Data
	copy(canData[:], msg.Data)

	signals := make(map[string]any, len(message.Signals))
	for _, sig := range message.Signals {
		if desc, ok := sig.UnmarshalValueDescription(canData); ok {
			signals[sig.Name] = desc
		} else {
			signals[sig.Name] = sig.UnmarshalPhysical(canData)
		}
	}

	result := parseResult{
		Message: message.Name,
		Signals: signals,
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		dc.l.Error("failed to marshal parse result", "error", err)
		return nil
	}
	return string(jsonBytes)
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
