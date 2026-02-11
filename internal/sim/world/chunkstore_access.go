package world

import "sort"

func (s *ChunkStore) inBounds(pos Vec3i) bool {
	if pos.Y != 0 {
		return false
	}
	if s.gen.BoundaryR > 0 {
		if pos.X < -s.gen.BoundaryR || pos.X > s.gen.BoundaryR || pos.Z < -s.gen.BoundaryR || pos.Z > s.gen.BoundaryR {
			return false
		}
	}
	return true
}

func (s *ChunkStore) LoadedChunkKeys() []ChunkKey {
	keys := make([]ChunkKey, 0, len(s.chunks))
	for k := range s.chunks {
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

func (s *ChunkStore) GetBlock(pos Vec3i) uint16 {
	if pos.Y != 0 {
		return s.gen.Air
	}
	if !s.inBounds(pos) {
		return s.gen.Air
	}

	cx := floorDiv(pos.X, 16)
	cz := floorDiv(pos.Z, 16)
	lx := mod(pos.X, 16)
	lz := mod(pos.Z, 16)
	ch := s.getOrGenChunk(cx, cz)
	return ch.Get(lx, lz)
}

func (s *ChunkStore) SetBlock(pos Vec3i, b uint16) {
	if pos.Y != 0 {
		return
	}
	if !s.inBounds(pos) {
		return
	}
	cx := floorDiv(pos.X, 16)
	cz := floorDiv(pos.Z, 16)
	lx := mod(pos.X, 16)
	lz := mod(pos.Z, 16)
	ch := s.getOrGenChunk(cx, cz)
	ch.Set(lx, lz, b)
}

func (s *ChunkStore) getOrGenChunk(cx, cz int) *Chunk {
	k := ChunkKey{CX: cx, CZ: cz}
	if ch, ok := s.chunks[k]; ok {
		return ch
	}
	ch := &Chunk{
		CX:     cx,
		CZ:     cz,
		Blocks: make([]uint16, 16*16),
	}
	s.generateChunk(ch)
	ch.dirty = true
	_ = ch.Digest() // initialize digest
	s.chunks[k] = ch
	return ch
}
