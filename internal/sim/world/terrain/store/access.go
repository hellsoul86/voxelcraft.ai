package store

import (
	"sort"

	genpkg "voxelcraft.ai/internal/sim/world/terrain/gen"
)

func (s *ChunkStore) InBounds(x, y, z int) bool {
	if y != 0 {
		return false
	}
	if s.Gen.BoundaryR > 0 {
		if x < -s.Gen.BoundaryR || x > s.Gen.BoundaryR || z < -s.Gen.BoundaryR || z > s.Gen.BoundaryR {
			return false
		}
	}
	return true
}

func (s *ChunkStore) LoadedChunkKeys() []ChunkKey {
	keys := make([]ChunkKey, 0, len(s.Chunks))
	for k := range s.Chunks {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].CX != keys[j].CX {
			return keys[i].CX < keys[j].CX
		}
		return keys[i].CZ < keys[j].CZ
	})
	return keys
}

func (s *ChunkStore) GetBlock(x, y, z int) uint16 {
	if y != 0 || !s.InBounds(x, y, z) {
		return s.Gen.Air
	}

	cx := genpkg.FloorDiv(x, 16)
	cz := genpkg.FloorDiv(z, 16)
	lx := genpkg.Mod(x, 16)
	lz := genpkg.Mod(z, 16)
	ch := s.GetOrGenChunk(cx, cz)
	return ch.Get(lx, lz)
}

func (s *ChunkStore) SetBlock(x, y, z int, b uint16) {
	if y != 0 || !s.InBounds(x, y, z) {
		return
	}

	cx := genpkg.FloorDiv(x, 16)
	cz := genpkg.FloorDiv(z, 16)
	lx := genpkg.Mod(x, 16)
	lz := genpkg.Mod(z, 16)
	ch := s.GetOrGenChunk(cx, cz)
	ch.Set(lx, lz, b)
}

func (s *ChunkStore) GetOrGenChunk(cx, cz int) *Chunk {
	k := ChunkKey{CX: cx, CZ: cz}
	if ch, ok := s.Chunks[k]; ok {
		return ch
	}
	ch := &Chunk{
		CX:     cx,
		CZ:     cz,
		Blocks: make([]uint16, 16*16),
	}
	s.GenerateChunk(ch)
	ch.dirty = true
	_ = ch.Digest()
	s.Chunks[k] = ch
	return ch
}
