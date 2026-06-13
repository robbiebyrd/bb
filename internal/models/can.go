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

// OBDOptions configures an ELM327/STN OBD adapter interface. Env vars compose
// under the interface's OBD_ prefix — e.g. INTERFACE_0_OBD_MODE — and the fields
// nest under an "obd" object in JSON config.
type OBDOptions struct {
	// Mode selects how the adapter is driven: "monitor" (passive, default),
	// "poll" (active OBD-II PID polling), or "hybrid" (interleave monitor+poll,
	// STN devices only — ELM327 falls back to monitor/poll).
	Mode string `env:"MODE" envDefault:"monitor" json:"mode"`
	// Protocol is the ELM327 ATSP selector (e.g. "6" = ISO 15765-4 CAN 11-bit/500k,
	// "7" = 29-bit/500k, "0" = automatic). Empty defaults to "6".
	Protocol string `env:"PROTOCOL" envDefault:"" json:"protocol"`
	// HWFilters is a list of hex CAN identifiers the adapter passes through in
	// hardware (empty = all frames), e.g. "7E8,7E9,244". STN installs one filter
	// per id; ELM327 reduces them to a single code/mask.
	HWFilters []string `env:"HW_FILTER" envDefault:"" json:"hwFilter"`
	// PIDs is a list of OBD-II request strings to poll, e.g. "010C,010D,0105".
	// Required for poll and hybrid modes.
	PIDs []string `env:"PIDS" envDefault:"" json:"pids"`
	// PollMS is the poll interval in milliseconds for poll/hybrid modes.
	PollMS int `env:"POLL_MS" envDefault:"1000" json:"pollMs"`
	// PortBaud is the serial line speed. Nominal for Bluetooth rfcomm links.
	PortBaud int `env:"PORT_BAUD" envDefault:"115200" json:"portBaud"`
}

type CanInterfaceOption struct {
	Name     string   `env:"NAME,required" json:"name"`
	URI      string   `env:"URI"  envDefault:"" json:"uri"`
	Network  string   `env:"NET"  envDefault:"can" json:"net"`
	DBCFiles []string `env:"DBC"  envDefault:"" json:"dbc"`
	Loop     bool     `env:"LOOP" envDefault:"false" json:"loop"`
	// SignalFilters is a list of "field:op:value" rules applied to decoded signals.
	// SignalFilterOp controls how rules are combined: "and" (default) or "or".
	// SignalFilterMode controls semantics: "exclude" (default, matching signals are
	// dropped) or "include" (only matching signals are kept).
	SignalFilters    []string `env:"SIGNAL_FILTER"      envDefault:"" json:"signalFilter"`
	SignalFilterOp   string   `env:"SIGNAL_FILTER_OP"   envDefault:"and" json:"signalFilterOp"`
	SignalFilterMode string   `env:"SIGNAL_FILTER_MODE" envDefault:"exclude" json:"signalFilterMode"`

	// OBD holds adapter options for the elm327 / stn networks.
	OBD OBDOptions `envPrefix:"OBD_" json:"obd"`
}

type CanConnection interface {
	SetID(id int)
	GetName() string
	GetInterfaceName() string
	Open() error
	Close() error
	Receive(wg *sync.WaitGroup)
}
