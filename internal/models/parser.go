package models

type ParserInterface interface {
	// ParseSignals decodes all signals in message and returns one
	// CanSignalTimestamped per signal. timestamp and iface are propagated
	// directly from the originating CanMessageTimestamped.
	ParseSignals(message CanMessageData, timestamp int64, iface int) []CanSignalTimestamped
}
