package world

import (
	"math"
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
)

func TestComputeResourceDensity(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "OVERWORLD",
		WorldType:  "OVERWORLD",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       123,
		BoundaryR:  200,
	}, cats)
	if err != nil {
		t.Fatalf("new world: %v", err)
	}

	ch := w.chunks.getOrGenChunk(0, 0)
	air := w.catalogs.Blocks.Index["AIR"]
	coal := w.catalogs.Blocks.Index["COAL_ORE"]
	iron := w.catalogs.Blocks.Index["IRON_ORE"]
	for i := range ch.Blocks {
		ch.Blocks[i] = air
	}
	for i := 0; i < 64; i++ {
		ch.Blocks[i] = coal
	}
	for i := 64; i < 128; i++ {
		ch.Blocks[i] = iron
	}

	d := w.computeResourceDensity()
	if got := d["COAL_ORE"]; math.Abs(got-0.25) > 1e-9 {
		t.Fatalf("coal density mismatch: got %.6f want 0.25", got)
	}
	if got := d["IRON_ORE"]; math.Abs(got-0.25) > 1e-9 {
		t.Fatalf("iron density mismatch: got %.6f want 0.25", got)
	}
	if got := d["CRYSTAL_ORE"]; got != 0 {
		t.Fatalf("crystal density mismatch: got %.6f want 0", got)
	}
}
