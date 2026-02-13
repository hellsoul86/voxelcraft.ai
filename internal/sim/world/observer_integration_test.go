package world

import (
	"encoding/json"
	"testing"

	"voxelcraft.ai/internal/observerproto"
	streamspkg "voxelcraft.ai/internal/sim/world/feature/observer/stream"
)

func TestComputeSurfaceCellAt_surfaceUpdateCases(t *testing.T) {
	w := &World{
		chunks: &ChunkStore{
			gen: WorldGen{
				Air:       0,
				BoundaryR: 0,
			},
			chunks: map[ChunkKey]*Chunk{},
		},
	}

	ch := &Chunk{
		CX:     0,
		CZ:     0,
		Blocks: make([]uint16, 16*16),
	}
	w.chunks.chunks[ChunkKey{CX: 0, CZ: 0}] = ch

	const (
		air   uint16 = 0
		stone uint16 = 1
		wood  uint16 = 2
	)

	// Use a fixed cell within chunk (0,0).
	wx, wz := 3, 9
	lx, lz := wx, wz

	// Baseline: empty cell => AIR@0.
	if got := w.computeSurfaceCellAt(wx, wz); got != (surfaceCell{B: air, Y: 0}) {
		t.Fatalf("baseline surface = %+v, want air@0", got)
	}

	// Place a block: surface becomes STONE@0.
	ch.Set(lx, lz, stone)
	if got := w.computeSurfaceCellAt(wx, wz); got != (surfaceCell{B: stone, Y: 0}) {
		t.Fatalf("place surface = %+v, want stone@0", got)
	}

	// Replace the surface: surface becomes WOOD@0.
	ch.Set(lx, lz, wood)
	if got := w.computeSurfaceCellAt(wx, wz); got != (surfaceCell{B: wood, Y: 0}) {
		t.Fatalf("replace surface = %+v, want wood@0", got)
	}

	// Mine the surface: remove block => AIR@0.
	ch.Set(lx, lz, air)
	if got := w.computeSurfaceCellAt(wx, wz); got != (surfaceCell{B: air, Y: 0}) {
		t.Fatalf("mine surface = %+v, want air@0", got)
	}
}

func TestStepObserverChunksForClient_emitsChunkPatch(t *testing.T) {
	w := &World{
		chunks: &ChunkStore{
			gen: WorldGen{
				Air:       0,
				BoundaryR: 0,
			},
			chunks: map[ChunkKey]*Chunk{},
		},
	}

	ch := &Chunk{
		CX:     0,
		CZ:     0,
		Blocks: make([]uint16, 16*16),
	}
	w.chunks.chunks[ChunkKey{CX: 0, CZ: 0}] = ch

	const (
		stone uint16 = 1
		wood  uint16 = 2
	)

	wx, wz := 3, 9
	ch.Set(wx, wz, stone)

	dataOut := make(chan []byte, 8)
	c := &observerClient{
		ID:      "O1",
		TickOut: make(chan []byte, 1),
		DataOut: dataOut,
		Config: observerCfg{
			ChunkRadius: 1,
			MaxChunks:   1024,
		},
		Chunks: map[streamspkg.ChunkKey]*observerChunk{},
	}

	key := streamspkg.ChunkKey{CX: 0, CZ: 0}
	st := &observerChunk{
		Key:            key,
		LastWantedTick: 100,
		SentFull:       true,
		NeedsFull:      false,
		Surface:        w.computeChunkSurface(0, 0),
	}
	c.Chunks[key] = st

	// Apply the world change first (audit is recorded after mutation).
	ch.Set(wx, wz, wood)
	audits := []AuditEntry{
		{
			Tick:   100,
			Actor:  "A1",
			Action: "SET_BLOCK",
			Pos:    [3]int{wx, 0, wz},
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
	if cell.X != wx || cell.Z != wz || cell.Block != wood || cell.Y != 0 {
		t.Fatalf("unexpected cell: %+v", cell)
	}
}
