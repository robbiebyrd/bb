package common

import "slices"

func PadOrTrim(bb []byte, size int) []byte {
	l := len(bb)
	if l >= size {
		return bb[:size]
	}
	tmp := make([]byte, size)
	copy(tmp, bb)
	return tmp
}

func ArrayAllFalse(arr []bool) bool {
	return !ArrayContainsBool(arr, true)
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
