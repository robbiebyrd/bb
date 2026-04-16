package models

import (
	"net"
	"sync"
)

type CanMessageTimestamped struct {
	Timestamp int64
	Interface int
	ID        uint32
	Transmit  bool
	Remote    bool
	Length    uint8
	Data      []byte
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
}

type CanConnection interface {
	GetID() int
	SetID(id int)
	GetName() string
	GetInterfaceName() string
	SetName(name string)
	GetConnection() net.Conn
	SetConnection(conn net.Conn)
	GetNetwork() string
	SetNetwork(name string)
	GetURI() string
	SetURI(name string)
	Open() error
	Close() error
	IsOpen() bool
	Discontinue() error
	Receive(wg *sync.WaitGroup)
}

