package models

import "sync"

type CanMessageTimestamped struct {
	Timestamp int64
	Interface int
	ID        uint32
	Transmit  bool
	Remote    bool
	Length    uint8
	Data      []byte
}

type CanSignalTimestamped struct {
	Timestamp int64
	Interface int
	ID        uint32
	Message   string
	Signal    string
	Value     float64
	Unit      string
}

type CanInterfaceOptions []CanInterfaceOption

type CanInterfaceOption struct {
	Name    string   `env:"NAME,required"`
	URI     string   `env:"URI"              envDefault:""`
	Network string   `env:"NET"              envDefault:"can"`
	DBCFiles []string `env:"DBC"             envDefault:""`
	Loop    bool     `env:"LOOP"             envDefault:"false"`
	// SignalFilters is a list of "field:op:value" rules applied to decoded signals.
	// SignalFilterOp controls how rules are combined: "and" (default) or "or".
	// SignalFilterMode controls semantics: "exclude" (default, matching signals are
	// dropped) or "include" (only matching signals are kept).
	SignalFilters    []string `env:"SIGNAL_FILTER"      envDefault:""`
	SignalFilterOp   string   `env:"SIGNAL_FILTER_OP"   envDefault:"and"`
	SignalFilterMode string   `env:"SIGNAL_FILTER_MODE" envDefault:"exclude"`
}

type CanConnection interface {
	SetID(id int)
	GetName() string
	GetInterfaceName() string
	Open() error
	Close() error
	Receive(wg *sync.WaitGroup)
}
