package can

import (
	"net"
	"sync"

	"go.einride.tech/can"
)

type CanMessage struct {
	Timestamp int64
	Interface string
	Transmit  bool
	ID        uint32
	Length    uint8
	Remote    bool
	Data      []byte
}

type Config struct {
	InfluxHost        string               `env:"INFLUX_HOST,required"`
	InfluxToken       string               `env:"INFLUX_TOKEN,required"`
	InfluxDatabase    string               `env:"INFLUX_DATABASE" envDefault:"can_data"`
	InfluxTableName   string               `env:"INFLUX_TABLE" envDefault:"can_message"`
	CanInterfaces     []CanInterfaceOption `envPrefix:"INTERFACE"`
	BufferMessageSize int                  `eng:"BUFFER_MSG_SIZE" envDefault:"1024"`
}

type CanInterfaceOptions []CanInterfaceOption

type CanInterfaceOption struct {
	Name    string `env:"NAME,required"`
	URI     string `env:"URI" envDefault:""`
	Network string `env:"NET" envDefault:"can"`
}

type CanConnection interface {
	GetName() string
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
	Receive(wg *sync.WaitGroup) bool
}

type ReceiverInterface interface {
	Receive() bool
	Frame() can.Frame
	Err() error
	Close() error
}
