package common

import (
	"slices"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

// ShouldFilter returns true and the matching filter name if any filter rejects the message.
func ShouldFilter(filters map[string]canModels.FilterInterface, canMsg canModels.CanMessageTimestamped) (bool, *string) {
	for name, filter := range filters {
		if filter.Filter(canMsg) {
			return true, &name
		}
	}
	return false, nil
}

func PadOrTrim(bb []byte, size int) []byte {
	l := len(bb)
	if l >= size {
		return bb[:size]
	}
	tmp := make([]byte, size)
	copy(tmp, bb)
	return tmp
}

func ArrayAllTrue(arr []bool) bool {
	return !ArrayContainsBool(arr, false)
}

func ArrayContainsFalse(arr []bool) bool {
	return ArrayContainsBool(arr, false)
}

func ArrayContainsTrue(arr []bool) bool {
	return ArrayContainsBool(arr, true)
}

func ArrayContainsBool(arr []bool, value bool) bool {
	return slices.Contains(arr, value)
}
