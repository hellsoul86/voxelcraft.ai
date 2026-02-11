package ids

import (
	"strconv"
	"strings"
)

func MaxU64(a, b uint64) uint64 {
	if a >= b {
		return a
	}
	return b
}

func ParseUintAfterPrefix(prefix, id string) (uint64, bool) {
	if !strings.HasPrefix(id, prefix) {
		return 0, false
	}
	n, err := strconv.ParseUint(id[len(prefix):], 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func ParseLandNum(id string) (uint64, bool) {
	i := strings.LastIndexByte(id, '_')
	if i < 0 || i+1 >= len(id) {
		return 0, false
	}
	n, err := strconv.ParseUint(id[i+1:], 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
