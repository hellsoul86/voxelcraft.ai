package worldtest

import (
	"math"
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestTaskProgress_UsesRecipeAndBlueprintParams(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	t.Run("craft progress uses recipe time_ticks", func(t *testing.T) {
		rec := cats.Recipes.ByID["chest"]
		if rec.RecipeID == "" || rec.TimeTicks != 5 {
			t.Fatalf("expected chest recipe time_ticks=5, got %+v", rec)
		}

		h := NewHarness(t, world.WorldConfig{
			ID:           "test",
			Seed:         11,
			StarterItems: map[string]int{"PLANK": 8},
		}, cats, "crafter")
		h.StepNoop() // tick 1

		self := h.LastObs().Self.Pos
		if err := h.W.DebugSetBlock(world.Vec3i{X: self[0] + 1, Y: 0, Z: self[2]}, "CRAFTING_BENCH"); err != nil {
			t.Fatalf("DebugSetBlock(BENCH): %v", err)
		}

		h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "CRAFT", RecipeID: "chest", Count: 1}}, nil)
		h.StepNoop() // 2nd tick of work

		p := taskProgressByKind(h.LastObs().Tasks, "CRAFT")
		want := 2.0 / float64(rec.TimeTicks)
		if math.Abs(p-want) > 1e-6 {
			t.Fatalf("CRAFT progress=%v want %v", p, want)
		}
	})

	t.Run("smelt progress uses recipe time_ticks", func(t *testing.T) {
		timeTicks := 0
		for _, r := range cats.Recipes.ByID {
			if r.Station == "FURNACE" && len(r.Inputs) > 0 && r.Inputs[0].Item == "IRON_ORE" {
				timeTicks = r.TimeTicks
				break
			}
		}
		if timeTicks != 10 {
			t.Fatalf("expected IRON_ORE smelt time_ticks=10, got %d", timeTicks)
		}

		h := NewHarness(t, world.WorldConfig{
			ID:           "test",
			Seed:         12,
			StarterItems: map[string]int{"IRON_ORE": 1, "COAL": 1},
		}, cats, "smelter")
		h.StepNoop() // tick 1

		self := h.LastObs().Self.Pos
		if err := h.W.DebugSetBlock(world.Vec3i{X: self[0] + 1, Y: 0, Z: self[2]}, "FURNACE"); err != nil {
			t.Fatalf("DebugSetBlock(FURNACE): %v", err)
		}

		h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "SMELT", ItemID: "IRON_ORE", Count: 1}}, nil)
		h.StepNoop()
		h.StepNoop() // 3rd tick of work

		p := taskProgressByKind(h.LastObs().Tasks, "SMELT")
		want := 3.0 / float64(timeTicks)
		if math.Abs(p-want) > 1e-6 {
			t.Fatalf("SMELT progress=%v want %v", p, want)
		}
	})

	t.Run("blueprint progress uses build index", func(t *testing.T) {
		bp := cats.Blueprints.ByID["road_segment"]
		if bp.ID == "" || len(bp.Blocks) != 5 {
			t.Fatalf("unexpected road_segment blueprint: id=%q blocks=%d", bp.ID, len(bp.Blocks))
		}

		h := NewHarness(t, world.WorldConfig{
			ID:                     "test",
			Seed:                   13,
			BlueprintBlocksPerTick: 1,
			StarterItems:           map[string]int{"PLANK": 5},
		}, cats, "builder")
		h.StepNoop() // tick 1

		self := h.LastObs().Self.Pos
		anchor := world.Vec3i{X: self[0] + 4, Y: 0, Z: self[2]}
		clearArea(t, h, anchor, 8)

		h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "BUILD_BLUEPRINT", BlueprintID: "road_segment", Anchor: anchor.ToArray(), Rotation: 0}}, nil)
		p := taskProgressByKind(h.LastObs().Tasks, "BUILD_BLUEPRINT")
		if p <= 0 || p > 1 {
			t.Fatalf("BUILD_BLUEPRINT progress=%v want in (0,1]", p)
		}
		want := 1.0 / float64(len(bp.Blocks))
		if math.Abs(p-want) > 1e-6 {
			t.Fatalf("BUILD_BLUEPRINT progress=%v want %v", p, want)
		}
	})
}

func taskProgressByKind(tasksObs []protocol.TaskObs, kind string) float64 {
	for _, t := range tasksObs {
		if t.Kind == kind {
			return t.Progress
		}
	}
	return 0
}
