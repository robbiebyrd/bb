package models

type OutputClient interface {
	Run() error
	HandleCanMessage(canMsg CanMessageTimestamped)
	HandleCanMessageChannel() error
	GetChannel() chan CanMessageTimestamped
	GetName() string
	AddFilter(name string, filter FilterInterface) error
}
