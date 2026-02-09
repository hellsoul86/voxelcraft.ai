package world

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
)

func TestSnapshotExportImport_RoundTripDigest(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	cfg := WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     64,
		Seed:       42,
		BoundaryR:  4000,
	}

	w1, err := New(cfg, cats)
	if err != nil {
		t.Fatalf("world1: %v", err)
	}

	// Join one agent and make a few deterministic changes.
	out := make(chan []byte, 1)
	resp := make(chan JoinResponse, 1)
	w1.handleJoin(JoinRequest{Name: "bot", DeltaVoxels: false, Out: out, Resp: resp})
	jr := <-resp
	a := w1.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	// Place one block in the world directly.
	stoneID := w1.catalogs.Blocks.Index["STONE"]
	w1.chunks.SetBlock(a.Pos, stoneID)

	// Advance a few ticks (no actions).
	for i := 0; i < 10; i++ {
		w1.step(nil, nil, nil)
	}

	snapTick := w1.CurrentTick() - 1
	d1 := w1.stateDigest(snapTick)
	snap := w1.ExportSnapshot(snapTick)

	w2, err := New(cfg, cats)
	if err != nil {
		t.Fatalf("world2: %v", err)
	}
	if err := w2.ImportSnapshot(snap); err != nil {
		t.Fatalf("import: %v", err)
	}
	if got, want := w2.CurrentTick(), snapTick+1; got != want {
		t.Fatalf("tick after import: got %d want %d", got, want)
	}
	d2 := w2.stateDigest(snapTick)
	if d1 != d2 {
		t.Fatalf("digest mismatch after import: %s vs %s", d1, d2)
	}
}
