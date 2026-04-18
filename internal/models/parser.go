package models

type ParserInterface interface {
	// ParseSignals decodes all signals in message and returns one
	// CanSignalTimestamped per signal.
	ParseSignals(message CanMessageTimestamped) []CanSignalTimestamped
}
