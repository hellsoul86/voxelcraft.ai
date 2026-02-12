package world

import storepkg "voxelcraft.ai/internal/sim/world/terrain/store"

type ChunkKey = storepkg.ChunkKey
type Chunk = storepkg.Chunk
type WorldGen = storepkg.WorldGen

type ChunkStore struct {
	inner  *storepkg.ChunkStore
	gen    WorldGen
	chunks map[ChunkKey]*Chunk
}

func NewChunkStore(gen WorldGen) *ChunkStore {
	inner := storepkg.NewChunkStore(gen)
	return &ChunkStore{
		inner:  inner,
		gen:    inner.Gen,
		chunks: inner.Chunks,
	}
}

func (s *ChunkStore) inBounds(pos Vec3i) bool {
	if s == nil || s.inner == nil {
		return false
	}
	return s.inner.InBounds(pos.X, pos.Y, pos.Z)
}

func (s *ChunkStore) LoadedChunkKeys() []ChunkKey {
	if s == nil || s.inner == nil {
		return nil
	}
	return s.inner.LoadedChunkKeys()
}

func (s *ChunkStore) GetBlock(pos Vec3i) uint16 {
	if s == nil || s.inner == nil {
		return 0
	}
	return s.inner.GetBlock(pos.X, pos.Y, pos.Z)
}

func (s *ChunkStore) SetBlock(pos Vec3i, b uint16) {
	if s == nil || s.inner == nil {
		return
	}
	s.inner.SetBlock(pos.X, pos.Y, pos.Z, b)
}

func (s *ChunkStore) getOrGenChunk(cx, cz int) *Chunk {
	if s == nil || s.inner == nil {
		return nil
	}
	return s.inner.GetOrGenChunk(cx, cz)
}

func (s *ChunkStore) generateChunk(ch *Chunk) {
	if s == nil || s.inner == nil || ch == nil {
		return
	}
	s.inner.GenerateChunk(ch)
}
