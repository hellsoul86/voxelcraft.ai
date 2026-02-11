package digestcodec

import (
	"encoding/binary"
	"sort"
)

type mapWriter interface {
	Write(p []byte) (n int, err error)
}

// WriteSortedNonZeroIntMap emits a deterministic key-sorted map encoding,
// skipping zero values to keep digest payload stable and compact.
func WriteSortedNonZeroIntMap(w mapWriter, tmp *[8]byte, m map[string]int) {
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if v != 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		w.Write([]byte(k))
		binary.LittleEndian.PutUint64(tmp[:], uint64(m[k]))
		w.Write(tmp[:])
	}
}
