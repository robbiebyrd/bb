package common

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
