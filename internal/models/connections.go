package models

type ConnectionManager interface {
	Add(conn CanConnection)
	Connections() []CanConnection
	ConnectionByName(name string) CanConnection
	Connect(options CanInterfaceOption)
	ConnectMultiple(CanInterfaceOptions)
	DeleteConnection(name string)
	OpenAll() error
	CloseAll() error
	ReceiveAll() error
}
