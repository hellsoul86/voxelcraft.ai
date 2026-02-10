package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestConveyor_SensorGatesEnable(t *testing.T) {
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
	back := Vec3i{X: pos.X, Y: 0, Z: pos.Z - 1}
	front := Vec3i{X: pos.X, Y: 0, Z: pos.Z + 1}

	// Sensor is adjacent (east) of the conveyor.
	sensorPos := Vec3i{X: pos.X + 1, Y: 0, Z: pos.Z}
	// Dummy chest adjacent (east) of the sensor to toggle it ON via inventory presence.
	dummy := Vec3i{X: pos.X + 2, Y: 0, Z: pos.Z}

	w.chunks.SetBlock(pos, w.chunks.gen.Air)
	w.chunks.SetBlock(back, w.chunks.gen.Air)
	w.chunks.SetBlock(front, w.chunks.gen.Air)
	w.chunks.SetBlock(sensorPos, w.chunks.gen.Air)
	w.chunks.SetBlock(dummy, w.chunks.gen.Air)

	place := func(item string, at Vec3i, taskID string) {
		a.Inventory[item]++
		w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
			Type:            protocol.TypeAct,
			ProtocolVersion: protocol.Version,
			Tick:            w.CurrentTick(),
			Tasks:           []protocol.TaskReq{{ID: taskID, Type: "PLACE", ItemID: item, BlockPos: at.ToArray()}},
		}}})
	}

	place("CHEST", back, "K1")
	place("CHEST", front, "K2")
	place("CHEST", dummy, "K3")
	place("SENSOR", sensorPos, "K4")
	place("CONVEYOR", pos, "K5")

	cb := w.containers[back]
	cf := w.containers[front]
	cd := w.containers[dummy]
	if cb == nil || cf == nil || cd == nil {
		t.Fatalf("expected chest containers to exist: back=%v front=%v dummy=%v", cb, cf, cd)
	}

	// Put 2 coal in the back chest.
	cb.Inventory["COAL"] = 2

	// Sensor is OFF (dummy chest empty) -> conveyor should not pull/move anything.
	w.step(nil, nil, nil)
	w.step(nil, nil, nil)
	w.step(nil, nil, nil)
	if got := cf.Inventory["COAL"]; got != 0 {
		t.Fatalf("front chest coal=%d want 0 (sensor off)", got)
	}
	if got := cb.Inventory["COAL"]; got != 2 {
		t.Fatalf("back chest coal=%d want 2 (sensor off)", got)
	}

	// Turn sensor ON by adding any available inventory to the dummy chest.
	cd.Inventory["STONE"] = 1

	// Tick 1: pull 1 coal onto belt.
	w.step(nil, nil, nil)
	// Tick 2: insert 1 coal into front chest, pull the 2nd onto belt.
	w.step(nil, nil, nil)
	// Tick 3: insert 2nd coal.
	w.step(nil, nil, nil)

	if got := cf.Inventory["COAL"]; got != 2 {
		t.Fatalf("front chest coal=%d want 2 (sensor on)", got)
	}
	if got := cb.Inventory["COAL"]; got != 0 {
		t.Fatalf("back chest coal=%d want 0 (sensor on)", got)
	}
}
