package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestStationProximity2D_CraftBenchAdjacent(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	rec := cats.Recipes.ByID["chest"]
	if rec.RecipeID == "" || rec.TimeTicks <= 0 {
		t.Fatalf("missing chest recipe")
	}

	h := NewHarness(t, world.WorldConfig{ID: "test", Seed: 1, StarterItems: map[string]int{"PLANK": 8}}, cats, "crafter")
	h.StepNoop() // tick 1

	self := h.LastObs().Self.Pos
	benchPos := world.Vec3i{X: self[0] + 1, Y: 0, Z: self[2]}
	if err := h.W.DebugSetBlock(benchPos, "CRAFTING_BENCH"); err != nil {
		t.Fatalf("DebugSetBlock(BENCH): %v", err)
	}

	h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "CRAFT", RecipeID: "chest", Count: 1}}, nil)
	for i := 0; i < rec.TimeTicks+2; i++ {
		h.StepNoop()
	}

	obs := h.LastObs()
	if got := invCount(obs.Inventory, "PLANK"); got != 0 {
		t.Fatalf("plank not consumed: got %d want 0", got)
	}
	if got := invCount(obs.Inventory, "CHEST"); got != 1 {
		t.Fatalf("chest not crafted: got %d want 1", got)
	}
}

func TestStationProximity2D_SmeltFurnaceAdjacent(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	timeTicks := 0
	for _, r := range cats.Recipes.ByID {
		if r.Station == "FURNACE" && len(r.Inputs) > 0 && r.Inputs[0].Item == "IRON_ORE" {
			timeTicks = r.TimeTicks
			break
		}
	}
	if timeTicks <= 0 {
		t.Fatalf("missing smelt recipe for IRON_ORE")
	}

	h := NewHarness(t, world.WorldConfig{ID: "test", Seed: 2, StarterItems: map[string]int{"IRON_ORE": 1, "COAL": 1}}, cats, "smelter")
	h.StepNoop() // tick 1

	self := h.LastObs().Self.Pos
	furnacePos := world.Vec3i{X: self[0] + 1, Y: 0, Z: self[2]}
	if err := h.W.DebugSetBlock(furnacePos, "FURNACE"); err != nil {
		t.Fatalf("DebugSetBlock(FURNACE): %v", err)
	}

	h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "SMELT", ItemID: "IRON_ORE", Count: 1}}, nil)
	for i := 0; i < timeTicks+2; i++ {
		h.StepNoop()
	}

	obs := h.LastObs()
	if got := invCount(obs.Inventory, "IRON_ORE"); got != 0 {
		t.Fatalf("IRON_ORE=%d want 0", got)
	}
	if got := invCount(obs.Inventory, "COAL"); got != 0 {
		t.Fatalf("COAL=%d want 0", got)
	}
	if got := invCount(obs.Inventory, "IRON_INGOT"); got != 1 {
		t.Fatalf("IRON_INGOT=%d want 1", got)
	}
}
