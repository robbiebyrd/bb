package models

type OutputClient interface {
	Run() error
	Handle(canMsg CanMessage)
	HandleChannel() error
	GetChannel() chan CanMessage
	GetName() string
}
