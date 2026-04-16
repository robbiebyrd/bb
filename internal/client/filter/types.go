package filter

import (
	"strings"

	"github.com/robbiebyrd/bb/internal/client/common"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

type CanInterfaceFilter struct {
	Value    string
	Operator canModels.CanFilterTextOperator
}

func (iff CanInterfaceFilter) Filter(canMsg canModels.CanMessageTimestamped) bool {
	switch iff.Operator {
	case canModels.TextContains:
		return strings.Contains(canMsg.Interface, iff.Value)
	default:
		return canMsg.Interface == iff.Value
	}
}

type CanTransmitFilter struct {
	Value canModels.CanTransmitFilterStatus
}

func (tf CanTransmitFilter) Filter(canMsg canModels.CanMessageTimestamped) bool {
	switch tf.Value {
	case canModels.TXOnly:
		return canMsg.Transmit
	case canModels.RXOnly:
		return !canMsg.Transmit
	default:
		return true
	}
}

type CanDataLengthFilter struct {
	Value    uint8
	Operator canModels.CanDataLengthOperator
}

func (dlf CanDataLengthFilter) Filter(canMsg canModels.CanMessageTimestamped) bool {
	switch dlf.Operator {
	case canModels.LengthGreaterThan:
		return canMsg.Length > dlf.Value
	case canModels.LengthLessThan:
		return canMsg.Length < dlf.Value
	case canModels.LengthNotEquals:
		return canMsg.Length != dlf.Value
	default:
		return canMsg.Length == dlf.Value
	}
}

type CanRemoteFilter struct {
	Value canModels.CanRemoteFilterStatus
}

func (rf CanRemoteFilter) Filter(canMsg canModels.CanMessageTimestamped) bool {
	switch rf.Value {
	case canModels.ExcludeRemote:
		return !canMsg.Remote
	case canModels.RemoteOnly:
		return canMsg.Remote
	default:
		return true
	}
}

type CanDataFilter struct {
	Operator canModels.CanFilterGroupOperator
	Filters  []struct {
		Byte     uint8
		Operator canModels.CanDataFilterOperator
		Target   uint8
	}
}

func (df CanDataFilter) Filter(msg canModels.CanMessageTimestamped) bool {
	filterValues := []bool{}

	for _, f := range df.Filters {
		currentValue := false

		switch f.Operator {
		case canModels.DataGreaterThan:
			currentValue = msg.Data[f.Byte] > f.Target
		case canModels.DataLessThan:
			currentValue = msg.Data[f.Byte] < f.Target
		case canModels.DataNotEquals:
			currentValue = msg.Data[f.Byte] != f.Target
		default:
			currentValue = msg.Data[f.Byte] == f.Target
		}

		filterValues = append(filterValues, currentValue)
	}

	switch df.Operator {
	case canModels.FilterAnd:
		return common.ArrayAllTrue(filterValues)
	default:
		return common.ArrayContainsTrue(filterValues)
	}
}
