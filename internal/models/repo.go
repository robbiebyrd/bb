package models

type DBClient interface {
	Run() error
	Handle(msg CanMessage)
	HandleChannel(channel chan CanMessage)
}
