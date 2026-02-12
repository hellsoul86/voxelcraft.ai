package digest

import (
	"voxelcraft.ai/internal/sim/world/io/digestcodec"
)

type Writer interface {
	Write(p []byte) (n int, err error)
}

func BoolByte(b bool) byte { return digestcodec.BoolByte(b) }

func Clamp01(x float64) float64 { return digestcodec.Clamp01(x) }

func WriteSortedNonZeroIntMap(w Writer, tmp *[8]byte, m map[string]int) {
	digestcodec.WriteSortedNonZeroIntMap(w, tmp, m)
}
