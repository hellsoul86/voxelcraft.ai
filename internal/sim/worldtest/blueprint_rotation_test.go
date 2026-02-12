package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestBuildBlueprint_RotationAffectsPlacementAndValidation(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:           "test",
		TickRateHz:   5,
		DayTicks:     6000,
		ObsRadius:    7,
		Height:       1,
		Seed:         42,
		BoundaryR:    4000,
		StarterItems: map[string]int{"PLANK": 50},
	}, cats, "builder")

	anchor := world.Vec3i{X: 10, Y: 0, Z: 10}
	rot := 1
	clearArea(t, h, anchor, 8)

	// road_segment places 5 blocks at 2 blocks/tick => 3 ticks total including start tick.
	h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "BUILD_BLUEPRINT", BlueprintID: "road_segment", Anchor: anchor.ToArray(), Rotation: rot}}, nil)
	h.StepNoop()
	h.StepNoop()

	plankID := cats.Blocks.Index["PLANK"]
	// rotation=1 should place along +X axis: offsets (0..4, 0)
	for i := 0; i < 5; i++ {
		p := world.Vec3i{X: anchor.X + i, Y: 0, Z: anchor.Z}
		got, err := h.W.DebugGetBlock(p)
		if err != nil {
			t.Fatalf("DebugGetBlock: %v", err)
		}
		if got != plankID {
			t.Fatalf("block at %+v: got %d want %d (PLANK)", p, got, plankID)
		}
	}

	if !h.W.CheckBlueprintPlaced("road_segment", anchor.ToArray(), rot) {
		t.Fatalf("expected rotated blueprint validation to pass")
	}
	if h.W.CheckBlueprintPlaced("road_segment", anchor.ToArray(), 0) {
		t.Fatalf("expected unrotated blueprint validation to fail")
	}
}

func TestBuildBlueprint_Rotation_LShape(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:           "test",
		TickRateHz:   5,
		DayTicks:     6000,
		ObsRadius:    7,
		Height:       1,
		Seed:         42,
		BoundaryR:    4000,
		StarterItems: map[string]int{"PLANK": 50},
	}, cats, "builder")

	anchor := world.Vec3i{X: 10, Y: 0, Z: 10}
	rot := 1
	clearArea(t, h, anchor, 8)

	// road_turn places 8 blocks at 2 blocks/tick => 4 ticks total including start tick.
	h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "BUILD_BLUEPRINT", BlueprintID: "road_turn", Anchor: anchor.ToArray(), Rotation: rot}}, nil)
	h.StepNoop()
	h.StepNoop()
	h.StepNoop()

	plankID := cats.Blocks.Index["PLANK"]
	// rotation=1 should place:
	// - a 5-long segment along +X at Z=0 (relative), and
	// - a 3-long segment along -Z at X=4 (relative), excluding the corner overlap.
	expect := []world.Vec3i{
		{X: anchor.X + 0, Y: 0, Z: anchor.Z + 0},
		{X: anchor.X + 1, Y: 0, Z: anchor.Z + 0},
		{X: anchor.X + 2, Y: 0, Z: anchor.Z + 0},
		{X: anchor.X + 3, Y: 0, Z: anchor.Z + 0},
		{X: anchor.X + 4, Y: 0, Z: anchor.Z + 0},
		{X: anchor.X + 4, Y: 0, Z: anchor.Z - 1},
		{X: anchor.X + 4, Y: 0, Z: anchor.Z - 2},
		{X: anchor.X + 4, Y: 0, Z: anchor.Z - 3},
	}
	for _, p := range expect {
		got, err := h.W.DebugGetBlock(p)
		if err != nil {
			t.Fatalf("DebugGetBlock: %v", err)
		}
		if got != plankID {
			t.Fatalf("block at %+v: got %d want %d (PLANK)", p, got, plankID)
		}
	}

	if !h.W.CheckBlueprintPlaced("road_turn", anchor.ToArray(), rot) {
		t.Fatalf("expected rotated blueprint validation to pass")
	}
	if h.W.CheckBlueprintPlaced("road_turn", anchor.ToArray(), 0) {
		t.Fatalf("expected unrotated blueprint validation to fail")
	}
}

func clearArea(t *testing.T, h *Harness, center world.Vec3i, radius int) {
	t.Helper()
	for dz := -radius; dz <= radius; dz++ {
		for dx := -radius; dx <= radius; dx++ {
			if err := h.W.DebugSetBlock(world.Vec3i{X: center.X + dx, Y: 0, Z: center.Z + dz}, "AIR"); err != nil {
				t.Fatalf("DebugSetBlock: %v", err)
			}
		}
	}
}
