package store

import (
	"testing"

	snapv1 "voxelcraft.ai/internal/persistence/snapshot"
)

func TestExportAndImportChunksRoundTrip(t *testing.T) {
	gen := WorldGen{Seed: 7, Air: 0}
	s := NewChunkStore(gen)
	ch := &Chunk{
		CX:     1,
		CZ:     -2,
		Blocks: make([]uint16, 16*16),
	}
	ch.Blocks[0] = 3
	ch.Blocks[17] = 9
	s.Chunks[ChunkKey{CX: ch.CX, CZ: ch.CZ}] = ch

	keys := []ChunkKey{{CX: 1, CZ: -2}}
	exported := ExportLoadedChunks(s.Chunks, keys)
	if len(exported) != 1 {
		t.Fatalf("expected 1 exported chunk, got %d", len(exported))
	}
	if exported[0].Blocks[0] != 3 || exported[0].Blocks[17] != 9 {
		t.Fatalf("unexpected exported blocks: got %d,%d", exported[0].Blocks[0], exported[0].Blocks[17])
	}

	imported, err := ImportChunks(gen, exported)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	got := imported.Chunks[ChunkKey{CX: 1, CZ: -2}]
	if got == nil {
		t.Fatalf("missing imported chunk")
	}
	if got.Blocks[0] != 3 || got.Blocks[17] != 9 {
		t.Fatalf("unexpected imported blocks: got %d,%d", got.Blocks[0], got.Blocks[17])
	}
}

func TestImportChunksRejectsInvalidShape(t *testing.T) {
	gen := WorldGen{Seed: 1, Air: 0}
	_, err := ImportChunks(gen, []snapv1.ChunkV1{{
		CX:     0,
		CZ:     0,
		Height: 2,
		Blocks: make([]uint16, 16*16),
	}})
	if err == nil {
		t.Fatalf("expected error for invalid chunk shape")
	}
}
