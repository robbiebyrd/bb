package models

type OutputClient interface {
	Run() error
	HandleChannel() error
	GetChannel() chan CanMessageTimestamped
	GetName() string
	AddFilter(name string, filter FilterInterface) error
}
