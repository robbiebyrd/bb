package common

import (
	canModels "github.com/robbiebyrd/bb/internal/models/can"
)

func InterfaceName(scc canModels.CanConnection) string {
	return scc.GetName() + ":>" + scc.GetNetwork() + ":>" + scc.GetURI()
}

func PadOrTrim(bb []byte, size int) []byte {
	l := len(bb)
	if l == size {
		return bb
	}
	if l > size {
		return bb[:size]
	}
	tmp := make([]byte, size)
	copy(tmp[:size-l], bb)
	return tmp
}
