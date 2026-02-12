package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestSmelt_ConfigDrivenRecipes(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	findSmeltTime := func(inputItem string) int {
		for _, r := range cats.Recipes.ByID {
			if r.Station == "FURNACE" && len(r.Inputs) > 0 && r.Inputs[0].Item == inputItem {
				return r.TimeTicks
			}
		}
		return 0
	}

	t.Run("unsupported item rejected at task start", func(t *testing.T) {
		h := NewHarness(t, world.WorldConfig{
			ID:           "test",
			Seed:         9,
			StarterItems: map[string]int{},
		}, cats, "smelter")
		h.StepNoop() // avoid tick-0 scripts

		if ok := h.W.DebugSetAgentVitals(h.DefaultAgentID, -1, 0, 1000); !ok {
			t.Fatalf("DebugSetAgentVitals returned false")
		}
		self := h.LastObs().Self.Pos
		if err := h.W.DebugSetBlock(world.Vec3i{X: self[0] + 1, Y: 0, Z: self[2]}, "FURNACE"); err != nil {
			t.Fatalf("DebugSetBlock(FURNACE): %v", err)
		}

		h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "SMELT", ItemID: "PLANK", Count: 1}}, nil)
		obs := h.LastObs()

		for _, task := range obs.Tasks {
			if task.Kind == "SMELT" {
				t.Fatalf("expected no work task for unsupported smelt item; tasks=%v", obs.Tasks)
			}
		}

		found := false
		for _, e := range obs.Events {
			if e["type"] != "ACTION_RESULT" || e["ref"] != "K1" {
				continue
			}
			found = true
			if ok, _ := e["ok"].(bool); ok {
				t.Fatalf("expected ok=false for unsupported smelt")
			}
			if code, _ := e["code"].(string); code != "E_INVALID_TARGET" {
				t.Fatalf("expected E_INVALID_TARGET, got %v", e["code"])
			}
			break
		}
		if !found {
			t.Fatalf("expected ACTION_RESULT for rejected SMELT task; events=%v", obs.Events)
		}
	})

	t.Run("iron ore smelt uses recipe time/inputs/outputs", func(t *testing.T) {
		timeTicks := findSmeltTime("IRON_ORE")
		if timeTicks <= 0 {
			t.Fatalf("missing smelt recipe for IRON_ORE")
		}

		h := NewHarness(t, world.WorldConfig{
			ID:           "test",
			Seed:         9,
			StarterItems: map[string]int{"IRON_ORE": 1, "COAL": 1},
		}, cats, "smelter")
		h.StepNoop()

		self := h.LastObs().Self.Pos
		if err := h.W.DebugSetBlock(world.Vec3i{X: self[0] + 1, Y: 0, Z: self[2]}, "FURNACE"); err != nil {
			t.Fatalf("DebugSetBlock(FURNACE): %v", err)
		}

		h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "SMELT", ItemID: "IRON_ORE", Count: 1}}, nil)

		// Not done before time_ticks.
		for i := 0; i < timeTicks-2; i++ {
			h.StepNoop()
		}
		obs := h.LastObs()
		if got := invCount(obs.Inventory, "IRON_INGOT"); got != 0 {
			t.Fatalf("smelt too early: IRON_INGOT=%d want 0", got)
		}
		if got := invCount(obs.Inventory, "IRON_ORE"); got != 1 {
			t.Fatalf("inputs consumed too early: IRON_ORE=%d want 1", got)
		}

		// Completion tick.
		h.StepNoop()
		obs = h.LastObs()
		if got := invCount(obs.Inventory, "IRON_ORE"); got != 0 {
			t.Fatalf("IRON_ORE=%d want 0", got)
		}
		if got := invCount(obs.Inventory, "COAL"); got != 0 {
			t.Fatalf("COAL=%d want 0", got)
		}
		if got := invCount(obs.Inventory, "IRON_INGOT"); got != 1 {
			t.Fatalf("IRON_INGOT=%d want 1", got)
		}
	})

	t.Run("raw meat smelt to cooked meat", func(t *testing.T) {
		timeTicks := findSmeltTime("RAW_MEAT")
		if timeTicks <= 0 {
			t.Fatalf("missing smelt recipe for RAW_MEAT")
		}

		h := NewHarness(t, world.WorldConfig{
			ID:           "test",
			Seed:         9,
			StarterItems: map[string]int{"RAW_MEAT": 1, "COAL": 1},
		}, cats, "smelter")
		h.StepNoop()

		self := h.LastObs().Self.Pos
		if err := h.W.DebugSetBlock(world.Vec3i{X: self[0] + 1, Y: 0, Z: self[2]}, "FURNACE"); err != nil {
			t.Fatalf("DebugSetBlock(FURNACE): %v", err)
		}

		h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "SMELT", ItemID: "RAW_MEAT", Count: 1}}, nil)

		for i := 0; i < timeTicks-2; i++ {
			h.StepNoop()
		}
		obs := h.LastObs()
		if got := invCount(obs.Inventory, "COOKED_MEAT"); got != 0 {
			t.Fatalf("smelt too early: COOKED_MEAT=%d want 0", got)
		}

		h.StepNoop()
		obs = h.LastObs()
		if got := invCount(obs.Inventory, "RAW_MEAT"); got != 0 {
			t.Fatalf("RAW_MEAT=%d want 0", got)
		}
		if got := invCount(obs.Inventory, "COAL"); got != 0 {
			t.Fatalf("COAL=%d want 0", got)
		}
		if got := invCount(obs.Inventory, "COOKED_MEAT"); got != 1 {
			t.Fatalf("COOKED_MEAT=%d want 1", got)
		}
	})
}
