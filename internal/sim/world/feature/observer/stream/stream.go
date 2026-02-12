package stream

import (
	"math"

	chunkspkg "voxelcraft.ai/internal/sim/world/feature/observer/chunks"
)

type ChunkKey struct {
	CX int
	CZ int
}

func ComputeWantedChunks(agents []ChunkKey, radius int, maxChunks int) []ChunkKey {
	in := make([]chunkspkg.Key, 0, len(agents))
	for _, k := range agents {
		in = append(in, chunkspkg.Key{CX: k.CX, CZ: k.CZ})
	}
	keys := chunkspkg.ComputeWantedChunks(in, radius, maxChunks)
	out := make([]ChunkKey, 0, len(keys))
	for _, k := range keys {
		out = append(out, ChunkKey{CX: k.CX, CZ: k.CZ})
	}
	return out
}

func ClampInt(v, min, max, def int) int {
	return chunkspkg.ClampInt(v, min, max, def)
}

func Ceil(v float64) float64 {
	return math.Ceil(v)
}
