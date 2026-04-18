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

type CanMessageData struct {
	Interface int
	ID        uint32
	Transmit  bool
	Remote    bool
	Length    uint8
	Data      []byte
}

type CanInterfaceOptions []CanInterfaceOption

type CanInterfaceOption struct {
	Name    string `env:"NAME,required"`
	URI     string `env:"URI"           envDefault:""`
	Network string `env:"NET"           envDefault:"can"`
	DBCFile string `env:"DBC"           envDefault:""`
	Loop    bool   `env:"LOOP"          envDefault:"false"`
}

type CanConnection interface {
	SetID(id int)
	GetName() string
	GetInterfaceName() string
	Open() error
	Close() error
	Receive(wg *sync.WaitGroup)
}
