package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestRecipesReachability_CraftPlaceToggleSwitch(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:           "test",
		Seed:         42,
		StarterItems: map[string]int{"CRAFTING_BENCH": 1, "WIRE": 1, "PLANK": 1},
	}, cats, "bot")
	h.StepNoop() // tick 1

	self := h.LastObs().Self.Pos
	benchPos := world.Vec3i{X: self[0] + 1, Y: 0, Z: self[2]}
	if err := h.W.DebugSetBlock(benchPos, "AIR"); err != nil {
		t.Fatalf("DebugSetBlock(AIR): %v", err)
	}

	// Place the crafting bench.
	h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "PLACE", ItemID: "CRAFTING_BENCH", BlockPos: benchPos.ToArray()}}, nil)
	got, err := h.W.DebugGetBlock(benchPos)
	if err != nil {
		t.Fatalf("DebugGetBlock: %v", err)
	}
	if want := cats.Blocks.Index["CRAFTING_BENCH"]; got != want {
		t.Fatalf("bench not placed: got %d want %d", got, want)
	}

	// Craft SWITCH via recipe (requires bench nearby).
	h.Step(nil, []protocol.TaskReq{{ID: "K2", Type: "CRAFT", RecipeID: "switch", Count: 1}}, nil)
	for i := 0; i < 6; i++ { // time_ticks=5, step a bit past completion
		h.StepNoop()
	}
	obs := h.LastObs()
	if got := invCount(obs.Inventory, "WIRE"); got != 0 {
		t.Fatalf("wire not consumed: got %d want %d", got, 0)
	}
	if got := invCount(obs.Inventory, "PLANK"); got != 0 {
		t.Fatalf("plank not consumed: got %d want %d", got, 0)
	}
	if got := invCount(obs.Inventory, "SWITCH"); got != 1 {
		t.Fatalf("switch not crafted: got %d want %d", got, 1)
	}

	// Place the switch.
	switchPos := world.Vec3i{X: self[0], Y: 0, Z: self[2] + 1}
	if err := h.W.DebugSetBlock(switchPos, "AIR"); err != nil {
		t.Fatalf("DebugSetBlock(AIR): %v", err)
	}
	h.Step(nil, []protocol.TaskReq{{ID: "K3", Type: "PLACE", ItemID: "SWITCH", BlockPos: switchPos.ToArray()}}, nil)

	obs = h.LastObs()
	switchEntityID := ""
	state := ""
	for _, e := range obs.Entities {
		if e.Type != "SWITCH" || e.Pos != switchPos.ToArray() {
			continue
		}
		switchEntityID = e.ID
		for _, tag := range e.Tags {
			if len(tag) >= len("state:") && tag[:len("state:")] == "state:" {
				state = tag[len("state:"):]
			}
		}
		break
	}
	if switchEntityID == "" {
		t.Fatalf("expected SWITCH entity at %+v; entities=%v", switchPos, obs.Entities)
	}
	if state != "off" {
		t.Fatalf("expected default switch state off, got %q", state)
	}

	// Toggle it.
	h.Step([]protocol.InstantReq{{ID: "I1", Type: "TOGGLE_SWITCH", TargetID: switchEntityID}}, nil, nil)
	obs = h.LastObs()
	state = ""
	for _, e := range obs.Entities {
		if e.Type != "SWITCH" || e.ID != switchEntityID {
			continue
		}
		for _, tag := range e.Tags {
			if len(tag) >= len("state:") && tag[:len("state:")] == "state:" {
				state = tag[len("state:"):]
			}
		}
		break
	}
	if state != "on" {
		t.Fatalf("expected switch to be on after toggle, got %q", state)
	}
}
