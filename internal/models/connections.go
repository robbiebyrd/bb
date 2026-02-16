package models

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
