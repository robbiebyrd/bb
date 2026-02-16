package models

type ParserInterface interface {
	Parse(message CanMessageData) any
}
