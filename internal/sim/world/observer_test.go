package world

import (
	"encoding/json"
	"testing"

	"voxelcraft.ai/internal/observerproto"
)

func TestComputeSurfaceCellAt_surfaceUpdateCases(t *testing.T) {
	w := &World{
		chunks: &ChunkStore{
			gen: WorldGen{
				Height:    8,
				Air:       0,
				BoundaryR: 0,
			},
			chunks: map[ChunkKey]*Chunk{},
		},
	}

	ch := &Chunk{
		CX:     0,
		CZ:     0,
		Height: 8,
		Blocks: make([]uint16, 16*16*8),
	}
	w.chunks.chunks[ChunkKey{CX: 0, CZ: 0}] = ch

	const (
		air   uint16 = 0
		stone uint16 = 1
		wood  uint16 = 2
		brick uint16 = 3
	)

	// Use a fixed column within chunk (0,0).
	wx, wz := 3, 9
	lx, lz := wx, wz

	// Baseline: surface is STONE at y=2.
	ch.Set(lx, 2, lz, stone)
	if got := w.computeSurfaceCellAt(wx, wz); got != (surfaceCell{b: stone, y: 2}) {
		t.Fatalf("baseline surface = %+v, want stone@2", got)
	}

	// Place a higher block: surface becomes WOOD at y=5.
	ch.Set(lx, 5, lz, wood)
	if got := w.computeSurfaceCellAt(wx, wz); got != (surfaceCell{b: wood, y: 5}) {
		t.Fatalf("place higher surface = %+v, want wood@5", got)
	}

	// Replace the surface at same height: surface becomes BRICK at y=5.
	ch.Set(lx, 5, lz, brick)
	if got := w.computeSurfaceCellAt(wx, wz); got != (surfaceCell{b: brick, y: 5}) {
		t.Fatalf("replace same-height surface = %+v, want brick@5", got)
	}

	// Mine the surface: remove y=5 and ensure we scan down to y=4.
	ch.Set(lx, 4, lz, stone)
	ch.Set(lx, 5, lz, air)
	if got := w.computeSurfaceCellAt(wx, wz); got != (surfaceCell{b: stone, y: 4}) {
		t.Fatalf("mine surface = %+v, want stone@4", got)
	}
}

func TestStepObserverChunksForClient_emitsChunkPatch(t *testing.T) {
	w := &World{
		chunks: &ChunkStore{
			gen: WorldGen{
				Height:    8,
				Air:       0,
				BoundaryR: 0,
			},
			chunks: map[ChunkKey]*Chunk{},
		},
	}

	ch := &Chunk{
		CX:     0,
		CZ:     0,
		Height: 8,
		Blocks: make([]uint16, 16*16*8),
	}
	w.chunks.chunks[ChunkKey{CX: 0, CZ: 0}] = ch

	const (
		stone uint16 = 1
		wood  uint16 = 2
	)

	wx, wz := 3, 9
	ch.Set(wx, 2, wz, stone)

	dataOut := make(chan []byte, 8)
	c := &observerClient{
		id:      "O1",
		tickOut: make(chan []byte, 1),
		dataOut: dataOut,
		cfg: observerCfg{
			chunkRadius: 1,
			maxChunks:   1024,
		},
		chunks: map[ChunkKey]*observerChunk{},
	}

	key := ChunkKey{CX: 0, CZ: 0}
	st := &observerChunk{
		key:            key,
		lastWantedTick: 100,
		sentFull:       true,
		needsFull:      false,
		surface:        w.computeChunkSurface(0, 0),
	}
	c.chunks[key] = st

	// Apply the world change first (audit is recorded after mutation).
	ch.Set(wx, 6, wz, wood)
	audits := []AuditEntry{
		{
			Tick:   100,
			Actor:  "A1",
			Action: "SET_BLOCK",
			Pos:    [3]int{wx, 6, wz},
			From:   stone,
			To:     wood,
			Reason: "TEST",
		},
	}

	w.stepObserverChunksForClient(100, c, nil, audits)

	var raw []byte
	select {
	case raw = <-dataOut:
	default:
		t.Fatalf("expected CHUNK_PATCH message to be enqueued")
	}

	var patch observerproto.ChunkPatchMsg
	if err := json.Unmarshal(raw, &patch); err != nil {
		t.Fatalf("unmarshal patch: %v raw=%s", err, string(raw))
	}
	if patch.Type != "CHUNK_PATCH" || patch.ProtocolVersion != observerproto.Version {
		t.Fatalf("unexpected patch header: %+v", patch)
	}
	if patch.CX != 0 || patch.CZ != 0 {
		t.Fatalf("unexpected chunk coords: cx=%d cz=%d", patch.CX, patch.CZ)
	}
	if len(patch.Cells) != 1 {
		t.Fatalf("unexpected cells len=%d", len(patch.Cells))
	}
	cell := patch.Cells[0]
	if cell.X != wx || cell.Z != wz || cell.Block != wood || cell.Y != 6 {
		t.Fatalf("unexpected cell: %+v", cell)
	}
}

