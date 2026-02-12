package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestConveyor_DisabledByAdjacentSwitchUntilToggledOn(t *testing.T) {
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

	a.Yaw = 0 // +Z
	pos := Vec3i{X: a.Pos.X, Y: 0, Z: a.Pos.Z}
	sw := Vec3i{X: pos.X + 1, Y: 0, Z: pos.Z}
	dst := Vec3i{X: pos.X, Y: 0, Z: pos.Z + 1}
	w.chunks.SetBlock(pos, w.chunks.gen.Air)
	w.chunks.SetBlock(sw, w.chunks.gen.Air)
	w.chunks.SetBlock(dst, w.chunks.gen.Air)

	// Place switch adjacent (defaults off).
	a.Inventory["SWITCH"] = 1
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "PLACE", ItemID: "SWITCH", BlockPos: sw.ToArray()}},
	}}})

	// Place conveyor.
	a.Inventory["CONVEYOR"] = 1
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K2", Type: "PLACE", ItemID: "CONVEYOR", BlockPos: pos.ToArray()}},
	}}})

	itemID := w.spawnItemEntity(w.CurrentTick(), "WORLD", pos, "IRON_INGOT", 1, "TEST")
	a.Pos = pos // ensure interaction range

	// With adjacent switch OFF, the belt should be disabled.
	w.step(nil, nil, nil)
	e := w.items[itemID]
	if e == nil {
		t.Fatalf("missing item entity")
	}
	if e.Pos != pos {
		t.Fatalf("item moved while switch off: pos=%+v want %+v", e.Pos, pos)
	}

	// Toggle switch ON; on the same tick, the belt should move the item.
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Instants:        []protocol.InstantReq{{ID: "I1", Type: "TOGGLE_SWITCH", TargetID: switchIDAt(sw)}},
	}}})
	e = w.items[itemID]
	if e == nil {
		t.Fatalf("missing item entity after toggle")
	}
	if e.Pos != dst {
		t.Fatalf("item pos=%+v want %+v", e.Pos, dst)
	}
}
