package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestConveyor_EnabledViaWirePoweredByRemoteSwitch(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     64,
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

	a.Yaw = 0 // +Z
	y := w.cfg.Height - 2
	conv := Vec3i{X: a.Pos.X, Y: y, Z: a.Pos.Z}
	dst := Vec3i{X: conv.X, Y: y, Z: conv.Z + 1}

	wire1 := Vec3i{X: conv.X + 1, Y: y, Z: conv.Z}
	wire2 := Vec3i{X: conv.X + 2, Y: y, Z: conv.Z}
	sw := Vec3i{X: conv.X + 3, Y: y, Z: conv.Z}

	// Place wires + remote switch + conveyor.
	a.Inventory["WIRE"] = 2
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "PLACE", ItemID: "WIRE", BlockPos: wire1.ToArray()}},
	}}})
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K2", Type: "PLACE", ItemID: "WIRE", BlockPos: wire2.ToArray()}},
	}}})

	a.Inventory["SWITCH"] = 1
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K3", Type: "PLACE", ItemID: "SWITCH", BlockPos: sw.ToArray()}},
	}}})

	a.Inventory["CONVEYOR"] = 1
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K4", Type: "PLACE", ItemID: "CONVEYOR", BlockPos: conv.ToArray()}},
	}}})

	itemID := w.spawnItemEntity(w.CurrentTick(), "WORLD", conv, "COAL", 1, "TEST")
	a.Pos = sw // for interaction range

	// With adjacent wire present and remote switch OFF, the belt should be disabled.
	w.step(nil, nil, nil)
	if e := w.items[itemID]; e == nil || e.Pos != conv {
		t.Fatalf("expected item to stay on disabled belt")
	}

	// Toggle remote switch ON -> belt should become enabled via wire network and move item.
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Instants:        []protocol.InstantReq{{ID: "I1", Type: "TOGGLE_SWITCH", TargetID: switchIDAt(sw)}},
	}}})
	if e := w.items[itemID]; e == nil || e.Pos != dst {
		t.Fatalf("expected item to move via powered wire network")
	}
}
