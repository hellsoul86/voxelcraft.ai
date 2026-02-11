package world

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"

	"voxelcraft.ai/internal/sim/world/io/digestcodec"
)

func (w *World) stateDigest(nowTick uint64) string {
	h := sha256.New()
	var tmp [8]byte

	w.digestHeader(h, &tmp, nowTick)
	w.digestChunks(h, &tmp)
	w.digestClaims(h, &tmp)
	w.digestLaws(h, &tmp)
	w.digestOrgs(h, &tmp)
	w.digestContainers(h, &tmp)
	w.digestItems(h, &tmp)
	w.digestSigns(h, &tmp)
	w.digestConveyors(h, &tmp)
	w.digestSwitches(h, &tmp)
	w.digestContracts(h, &tmp)
	w.digestTrades(h, &tmp)
	w.digestBoards(h, &tmp)
	w.digestStructures(h, &tmp)
	w.digestAgents(h, &tmp)

	return hex.EncodeToString(h.Sum(nil))
}

func digestWriteU64(h hashWriter, tmp *[8]byte, v uint64) {
	binary.LittleEndian.PutUint64(tmp[:], v)
	h.Write(tmp[:])
}

func digestWriteI64(h hashWriter, tmp *[8]byte, v int64) {
	digestWriteU64(h, tmp, uint64(v))
}

func boolByte(b bool) byte {
	return digestcodec.BoolByte(b)
}

func writeItemMap(h hashWriter, tmp [8]byte, m map[string]int) {
	digestcodec.WriteSortedNonZeroIntMap(h, &tmp, m)
}

type hashWriter interface {
	Write(p []byte) (n int, err error)
}

func clamp01(x float64) float64 {
	return digestcodec.Clamp01(x)
}
