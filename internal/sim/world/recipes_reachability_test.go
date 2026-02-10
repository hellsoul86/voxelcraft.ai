package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestRecipesReachability_CraftPlaceToggleSwitch(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	respCh := make(chan JoinResponse, 1)
	w.step([]JoinRequest{{Name: "bot", Resp: respCh}}, nil, nil)
	jr := <-respCh
	a := w.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	// Place a crafting bench at y=0 and position the agent next to it.
	benchPos := Vec3i{X: a.Pos.X + 1, Y: 0, Z: a.Pos.Z}
	a.Pos = Vec3i{X: a.Pos.X, Y: 0, Z: a.Pos.Z}
	// Ensure target cells are empty in the 2D world.
	w.chunks.SetBlock(benchPos, w.chunks.gen.Air)

	a.Inventory["CRAFTING_BENCH"] = 1
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "PLACE", ItemID: "CRAFTING_BENCH", BlockPos: benchPos.ToArray()}},
	}}})
	if got := w.blockName(w.chunks.GetBlock(benchPos)); got != "CRAFTING_BENCH" {
		t.Fatalf("bench not placed: got %q", got)
	}

	// Craft SWITCH via recipe (requires bench nearby).
	a.Inventory["WIRE"] = 1
	a.Inventory["PLANK"] = 1
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K2", Type: "CRAFT", RecipeID: "switch", Count: 1}},
	}}})
	// time_ticks=5, so advance a bit past completion.
	for i := 0; i < 6; i++ {
		w.step(nil, nil, nil)
	}
	if got := a.Inventory["WIRE"]; got != 0 {
		t.Fatalf("wire not consumed: got %d want %d", got, 0)
	}
	if got := a.Inventory["PLANK"]; got != 0 {
		t.Fatalf("plank not consumed: got %d want %d", got, 0)
	}
	if got := a.Inventory["SWITCH"]; got != 1 {
		t.Fatalf("switch not crafted: got %d want %d", got, 1)
	}

	// Place the switch and toggle it.
	switchPos := Vec3i{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z + 1}
	w.chunks.SetBlock(switchPos, w.chunks.gen.Air)
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K3", Type: "PLACE", ItemID: "SWITCH", BlockPos: switchPos.ToArray()}},
	}}})
	if on := w.switches[switchPos]; on {
		t.Fatalf("expected default switch state off")
	}

	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Instants:        []protocol.InstantReq{{ID: "I1", Type: "TOGGLE_SWITCH", TargetID: switchIDAt(switchPos)}},
	}}})
	if on := w.switches[switchPos]; !on {
		t.Fatalf("expected switch to be on after toggle")
	}
}
