package filter

import (
	"strings"

	"github.com/robbiebyrd/bb/internal/client/common"
	canModel "github.com/robbiebyrd/bb/internal/models"
)

type CanInterfaceFilter struct {
	Value    string
	Operator canModel.CanFilterTextOperator
}

func (iff CanInterfaceFilter) Filter(canMsg canModel.CanMessage) bool {
	switch iff.Operator {
	case canModel.TextContains:
		return strings.Contains(canMsg.Interface, iff.Value)
	default:
		return canMsg.Interface == iff.Value
	}
}

type CanTransmitFilter struct {
	Value canModel.CanTransmitFilterStatus
}

func (tf CanTransmitFilter) Filter(canMsg canModel.CanMessage) bool {
	switch tf.Value {
	case canModel.TXOnly:
		return canMsg.Transmit
	case canModel.RXOnly:
		return !canMsg.Transmit
	default:
		return true
	}
}

type CanDataLengthFilter struct {
	Value    uint8
	Operator canModel.CanDataLengthOperator
}

func (dlf CanDataLengthFilter) Filter(canMsg canModel.CanMessage) bool {
	switch dlf.Operator {
	case canModel.LengthGreaterThan:
		return canMsg.Length > dlf.Value
	case canModel.LengthLessThan:
		return canMsg.Length < dlf.Value
	case canModel.LengthNotEquals:
		return canMsg.Length != dlf.Value
	default:
		return canMsg.Length == dlf.Value
	}
}

type CanRemoteFilter struct {
	Value canModel.CanRemoteFilterStatus
}

func (rf CanRemoteFilter) Filter(canMsg canModel.CanMessage) bool {
	switch rf.Value {
	case canModel.ExcludeRemote:
		return !canMsg.Remote
	case canModel.RemoteOnly:
		return canMsg.Remote
	default:
		return true
	}
}

type CanDataFilter struct {
	Operator canModel.CanFilterGroupOperator
	Filters  []struct {
		Byte     uint8
		Operator canModel.CanDataFilterOperator
		Target   uint8
	}
}

func (df CanDataFilter) Filter(msg canModel.CanMessage) bool {
	filterValues := []bool{}

	for _, f := range df.Filters {
		currentValue := false

		switch f.Operator {
		case canModel.DataGreaterThan:
			currentValue = msg.Data[f.Byte] > f.Target
		case canModel.DataLessThan:
			currentValue = msg.Data[f.Byte] < f.Target
		case canModel.DataNotEquals:
			currentValue = msg.Data[f.Byte] != f.Target
		default:
			currentValue = msg.Data[f.Byte] == f.Target
		}

		filterValues = append(filterValues, currentValue)
	}

	switch df.Operator {
	case canModel.FilterAnd:
		return common.ArrayContainsFalse(filterValues)
	default:
		return common.ArrayContainsTrue(filterValues)
	}
}
