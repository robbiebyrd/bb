package models

// Network type identifiers for the NET interface option.
const (
	NetworkSim      = "sim"
	NetworkSLCAN    = "slcan"
	NetworkPlayback = "playback"
	NetworkCAN      = "can"
	NetworkELM327   = "elm327"
	NetworkSTN      = "stn"
)

type InterfaceResolver interface {
	ConnectionByID(id int) CanConnection
}

type ConnectionManager interface {
	InterfaceResolver
	Add(conn CanConnection) int
	Connections() []CanConnection
	ConnectionByName(name string) CanConnection
	Connect(options CanInterfaceOption)
	ConnectMultiple(CanInterfaceOptions)
	DeleteConnection(name string)
	OpenAll() error
	CloseAll() error
	ReceiveAll() error
}
