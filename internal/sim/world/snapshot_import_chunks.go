package world

import (
	"fmt"

	"voxelcraft.ai/internal/persistence/snapshot"
)

func (w *World) importChunkSnapshots(gen WorldGen, chunks []snapshot.ChunkV1) error {
	store := NewChunkStore(gen)
	for _, ch := range chunks {
		if ch.Height != 1 {
			return fmt.Errorf("snapshot chunk height mismatch: got %d want 1", ch.Height)
		}
		if len(ch.Blocks) != 16*16 {
			return fmt.Errorf("snapshot chunk blocks length mismatch: got %d want %d", len(ch.Blocks), 16*16)
		}
		k := ChunkKey{CX: ch.CX, CZ: ch.CZ}
		blocks := make([]uint16, len(ch.Blocks))
		copy(blocks, ch.Blocks)
		c := &Chunk{
			CX:     ch.CX,
			CZ:     ch.CZ,
			Blocks: blocks,
			dirty:  true,
		}
		_ = c.Digest()
		store.chunks[k] = c
	}
	w.chunks = store
	return nil
}
