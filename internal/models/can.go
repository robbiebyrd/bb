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
	Name     string   `env:"NAME,required"`
	URI      string   `env:"URI"              envDefault:""`
	Network  string   `env:"NET"              envDefault:"can"`
	DBCFiles []string `env:"DBC"             envDefault:""`
	Loop     bool     `env:"LOOP"             envDefault:"false"`
	// SignalFilters is a list of "field:op:value" rules applied to decoded signals.
	// SignalFilterOp controls how rules are combined: "and" (default) or "or".
	// SignalFilterMode controls semantics: "exclude" (default, matching signals are
	// dropped) or "include" (only matching signals are kept).
	SignalFilters    []string `env:"SIGNAL_FILTER"      envDefault:""`
	SignalFilterOp   string   `env:"SIGNAL_FILTER_OP"   envDefault:"and"`
	SignalFilterMode string   `env:"SIGNAL_FILTER_MODE" envDefault:"exclude"`

	// OBD adapter options (elm327 / stn networks).
	// OBDMode selects how the adapter is driven: "monitor" (passive, default),
	// "poll" (active OBD-II PID polling), or "hybrid" (interleave monitor+poll,
	// STN devices only — ELM327 falls back to monitor/poll).
	OBDMode string `env:"OBD_MODE" envDefault:"monitor"`
	// OBDProtocol is the ELM327 ATSP selector (e.g. "6" = ISO 15765-4 CAN
	// 11-bit/500k, "7" = 29-bit/500k, "0" = automatic). Empty defaults to "6".
	OBDProtocol string `env:"OBD_PROTOCOL" envDefault:""`
	// HWFilters is a comma-separated list of CAN identifiers the adapter should
	// pass through in hardware (empty = all frames). STN installs one filter per
	// id; ELM327 reduces them to a single code/mask pair.
	HWFilters []uint32 `env:"HW_FILTER" envDefault:""`
	// OBDPIDs is a comma-separated list of OBD-II request strings to poll, e.g.
	// "010C,010D,0105". Required for poll and hybrid modes.
	OBDPIDs []string `env:"OBD_PIDS" envDefault:""`
	// OBDPollMS is the poll interval in milliseconds for poll/hybrid modes.
	OBDPollMS int `env:"OBD_POLL_MS" envDefault:"1000"`
	// PortBaud is the serial line speed. Nominal for Bluetooth rfcomm links.
	PortBaud int `env:"PORT_BAUD" envDefault:"115200"`
}

type CanConnection interface {
	SetID(id int)
	GetName() string
	GetInterfaceName() string
	Open() error
	Close() error
	Receive(wg *sync.WaitGroup)
}
