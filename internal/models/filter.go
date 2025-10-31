package models

type CanTransmitFilterStatus int

const (
	TXAndRX CanTransmitFilterStatus = iota
	TXOnly
	RXOnly
)

type CanRemoteFilterStatus int

const (
	IncludeRemote CanRemoteFilterStatus = iota
	ExcludeRemote
	RemoteOnly
)

type CanFilterGroupOperator int

const (
	FilterAnd CanFilterGroupOperator = iota
	FilterOr
)

type CanFilterTextOperator int

const (
	TextContains CanFilterTextOperator = iota
	TextEquals
)

type CanDataLengthOperator int

const (
	LengthGreaterThan CanDataLengthOperator = iota
	LengthLessThan
	LengthEquals
	LengthNotEquals
)

type CanDataFilterOperator int

const (
	DataGreaterThan CanDataFilterOperator = iota
	DataLessThan
	DataEquals
	DataNotEquals
)

type CanMessageFilterGroup struct {
	Filters  []CanMessageFilter
	Operator CanFilterGroupOperator
}

type CanMessageFilter interface {
	Filter(canMsg CanMessage) bool
}
