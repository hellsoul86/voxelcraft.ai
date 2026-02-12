package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestSnapshotExportImport_RoundTripDigest(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	cfg := world.WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
	}

	h := NewHarness(t, cfg, cats, "bot")
	obs := h.LastObs()
	pos := world.Vec3i{X: obs.Self.Pos[0], Y: 0, Z: obs.Self.Pos[2]}

	// Place one block in the world directly.
	h.SetBlock(pos, "STONE")

	// Advance a few ticks (no actions).
	for i := 0; i < 10; i++ {
		h.StepNoop()
	}

	snapTick := h.W.CurrentTick() - 1
	d1 := h.W.DebugStateDigest(snapTick)
	snap := h.W.ExportSnapshot(snapTick)

	w2, err := world.New(cfg, cats)
	if err != nil {
		t.Fatalf("world2: %v", err)
	}
	if err := w2.ImportSnapshot(snap); err != nil {
		t.Fatalf("import: %v", err)
	}
	if got, want := w2.CurrentTick(), snapTick+1; got != want {
		t.Fatalf("tick after import: got %d want %d", got, want)
	}
	d2 := w2.DebugStateDigest(snapTick)
	if d1 != d2 {
		t.Fatalf("digest mismatch after import: %s vs %s", d1, d2)
	}
}
