package store

import (
	"fmt"

	snapv1 "voxelcraft.ai/internal/persistence/snapshot"
)

// ExportLoadedChunks converts loaded chunk data into snapshot chunks.
func ExportLoadedChunks(chunks map[ChunkKey]*Chunk, keys []ChunkKey) []snapv1.ChunkV1 {
	out := make([]snapv1.ChunkV1, 0, len(keys))
	for _, k := range keys {
		ch := chunks[k]
		if ch == nil {
			continue
		}
		blocks := make([]uint16, len(ch.Blocks))
		copy(blocks, ch.Blocks)
		out = append(out, snapv1.ChunkV1{
			CX:     k.CX,
			CZ:     k.CZ,
			Height: 1,
			Blocks: blocks,
		})
	}
	return out
}

// ImportChunks rebuilds a chunk store from snapshot chunks.
func ImportChunks(gen WorldGen, chunks []snapv1.ChunkV1) (*ChunkStore, error) {
	store := NewChunkStore(gen)
	for _, ch := range chunks {
		if ch.Height != 1 {
			return nil, fmt.Errorf("snapshot chunk height mismatch: got %d want 1", ch.Height)
		}
		if len(ch.Blocks) != 16*16 {
			return nil, fmt.Errorf("snapshot chunk blocks length mismatch: got %d want %d", len(ch.Blocks), 16*16)
		}
		k := ChunkKey{CX: ch.CX, CZ: ch.CZ}
		blocks := make([]uint16, len(ch.Blocks))
		copy(blocks, ch.Blocks)
		c := &Chunk{
			CX:     ch.CX,
			CZ:     ch.CZ,
			Blocks: blocks,
		}
		_ = c.Digest()
		store.Chunks[k] = c
	}
	return store, nil
}
