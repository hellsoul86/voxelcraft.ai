package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestToggleSwitch_TogglesState(t *testing.T) {
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

	// Place switch at y=0 and teleport onto it for interaction range checks.
	pos := Vec3i{X: a.Pos.X, Y: 0, Z: a.Pos.Z}
	w.chunks.SetBlock(pos, w.chunks.gen.Air)
	a.Inventory["SWITCH"] = 1
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "PLACE", ItemID: "SWITCH", BlockPos: pos.ToArray()}},
	}}})

	if on := w.switches[pos]; on {
		t.Fatalf("expected default switch state off")
	}

	a.Pos = pos
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Instants:        []protocol.InstantReq{{ID: "I1", Type: "TOGGLE_SWITCH", TargetID: switchIDAt(pos)}},
	}}})
	if on := w.switches[pos]; !on {
		t.Fatalf("expected switch to be on after toggle")
	}

	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Instants:        []protocol.InstantReq{{ID: "I2", Type: "TOGGLE_SWITCH", TargetID: switchIDAt(pos)}},
	}}})
	if on := w.switches[pos]; on {
		t.Fatalf("expected switch to be off after second toggle")
	}
}
