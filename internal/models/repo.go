package models

type OutputClient interface {
	Run() error
	HandleCanMessage(canMsg CanMessageTimestamped)
	HandleCanMessageChannel() error
	GetChannel() chan CanMessageTimestamped
	GetName() string
	AddFilter(name string, filter FilterInterface) error
}

// SignalOutputClient extends OutputClient for clients that consume decoded DBC
// signals. app.go detects this interface via type assertion at wiring time;
// clients that only process raw CAN frames need not implement it.
type SignalOutputClient interface {
	OutputClient
	HandleSignal(signal CanSignalTimestamped)
	HandleSignalChannel() error
	GetSignalChannel() chan CanSignalTimestamped
}
