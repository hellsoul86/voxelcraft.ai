package world

import "voxelcraft.ai/internal/persistence/snapshot"

func (w *World) exportChunkSnapshots() []snapshot.ChunkV1 {
	keys := w.chunks.LoadedChunkKeys()
	chunks := make([]snapshot.ChunkV1, 0, len(keys))
	for _, k := range keys {
		ch := w.chunks.chunks[k]
		blocks := make([]uint16, len(ch.Blocks))
		copy(blocks, ch.Blocks)
		chunks = append(chunks, snapshot.ChunkV1{
			CX:     k.CX,
			CZ:     k.CZ,
			Height: 1,
			Blocks: blocks,
		})
	}
	return chunks
}
