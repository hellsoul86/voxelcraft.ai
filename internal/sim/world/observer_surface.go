package world

import (
	"math"

	chunkspkg "voxelcraft.ai/internal/sim/world/feature/observer/chunks"
)

func (w *World) computeChunkSurface(cx, cz int) []surfaceCell {
	ch := w.chunkForSurface(cx, cz)
	air := w.chunks.gen.Air
	boundaryR := w.chunks.gen.BoundaryR
	out := make([]surfaceCell, 16*16)
	for z := 0; z < 16; z++ {
		for x := 0; x < 16; x++ {
			wx := cx*16 + x
			wz := cz*16 + z
			if boundaryR > 0 && (wx < -boundaryR || wx > boundaryR || wz < -boundaryR || wz > boundaryR) {
				out[z*16+x] = surfaceCell{b: air, y: 0}
				continue
			}
			b := air
			v := ch.Blocks[x+z*16]
			if v != air {
				b = v
			}
			out[z*16+x] = surfaceCell{b: b, y: 0}
		}
	}
	return out
}

func (w *World) computeChunkVoxels(cx, cz int) []uint16 {
	ch := w.chunkForVoxels(cx, cz)
	if ch == nil || ch.Blocks == nil {
		return nil
	}
	out := make([]uint16, len(ch.Blocks))
	copy(out, ch.Blocks)

	// Respect world boundary the same way ChunkStore.GetBlock does.
	if w.chunks != nil && w.chunks.gen.BoundaryR > 0 {
		air := w.chunks.gen.Air
		br := w.chunks.gen.BoundaryR
		for z := 0; z < 16; z++ {
			for x := 0; x < 16; x++ {
				wx := cx*16 + x
				wz := cz*16 + z
				if wx < -br || wx > br || wz < -br || wz > br {
					out[x+z*16] = air
				}
			}
		}
	}
	return out
}

func (w *World) computeSurfaceCellAt(wx, wz int) surfaceCell {
	air := w.chunks.gen.Air
	boundaryR := w.chunks.gen.BoundaryR
	if boundaryR > 0 && (wx < -boundaryR || wx > boundaryR || wz < -boundaryR || wz > boundaryR) {
		return surfaceCell{b: air, y: 0}
	}
	cx := floorDiv(wx, 16)
	cz := floorDiv(wz, 16)
	lx := mod(wx, 16)
	lz := mod(wz, 16)
	ch := w.chunkForSurface(cx, cz)
	v := ch.Blocks[lx+lz*16]
	if v != air {
		return surfaceCell{b: v, y: 0}
	}
	return surfaceCell{b: air, y: 0}
}

func (w *World) chunkForSurface(cx, cz int) *Chunk {
	if w == nil || w.chunks == nil {
		return &Chunk{CX: cx, CZ: cz, Blocks: nil}
	}
	k := ChunkKey{CX: cx, CZ: cz}
	if ch, ok := w.chunks.chunks[k]; ok && ch != nil {
		return ch
	}
	// Generate an ephemeral chunk without mutating the world's loaded chunk set. This ensures
	// observer clients cannot affect simulation state/digests by "viewing" far-away terrain.
	tmp := &Chunk{
		CX:     cx,
		CZ:     cz,
		Blocks: make([]uint16, 16*16),
	}
	w.chunks.generateChunk(tmp)
	return tmp
}

func (w *World) chunkForVoxels(cx, cz int) *Chunk {
	// Same semantics as chunkForSurface; kept separate to make intent explicit.
	return w.chunkForSurface(cx, cz)
}

func computeWantedChunks(agents []ChunkKey, radius int, maxChunks int) []ChunkKey {
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

func trySend(ch chan []byte, b []byte) bool {
	select {
	case ch <- b:
		return true
	default:
		return false
	}
}

func clampInt(v, min, max, def int) int {
	return chunkspkg.ClampInt(v, min, max, def)
}

// ceil is a tiny helper to avoid importing math in the world loop hot path.
func ceil(v float64) float64 {
	return math.Ceil(v)
}
