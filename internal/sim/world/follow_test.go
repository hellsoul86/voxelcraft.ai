package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/tasks"
)

func TestFollow_MovesTowardTargetAndMaintainsDistance(t *testing.T) {
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

	join := func(name string) *Agent {
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: name, DeltaVoxels: false, Out: nil, Resp: resp})
		j := <-resp
		return w.agents[j.Welcome.AgentID]
	}
	leader := join("leader")
	follower := join("follower")
	if leader == nil || follower == nil {
		t.Fatalf("missing agents")
	}

	leader.Pos = Vec3i{X: 0, Y: 0, Z: 0}
	follower.Pos = Vec3i{X: 10, Y: 0, Z: 0}

	// Clear a corridor so movement isn't blocked by generated solids.
	for x := 0; x <= 10; x++ {
		setAir(w, Vec3i{X: x, Y: 0, Z: 0})
	}

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         follower.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "FOLLOW", TargetID: leader.ID, Distance: 1.0},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: follower.ID, Act: act}})

	// Advance until follower should be within distance 1.
	for i := 0; i < 8; i++ {
		w.step(nil, nil, nil)
	}
	if got := follower.Pos.X; got != 1 {
		t.Fatalf("follower x: got %d want %d", got, 1)
	}

	// One more tick should not move closer than distance=1.
	w.step(nil, nil, nil)
	if got := follower.Pos.X; got != 1 {
		t.Fatalf("follower should hold distance: got x=%d want %d", got, 1)
	}
	if follower.MoveTask == nil || follower.MoveTask.Kind != tasks.KindFollow {
		t.Fatalf("expected follow task to remain active")
	}

	// Move leader away; follower should start moving again.
	leader.Pos = Vec3i{X: 5, Y: 0, Z: 0}
	w.step(nil, nil, nil)
	if got := follower.Pos.X; got != 2 {
		t.Fatalf("follower should advance after leader moved: got x=%d want %d", got, 2)
	}
}

func TestFollow_InvalidTargetFails(t *testing.T) {
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

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "follower", DeltaVoxels: false, Out: nil, Resp: resp})
	j := <-resp
	a := w.agents[j.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}
	a.Events = nil

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         a.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "FOLLOW", TargetID: "NOPE", Distance: 2.0},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})

	if a.MoveTask != nil {
		t.Fatalf("expected no movement task to be started")
	}
	found := false
	for _, e := range a.Events {
		if e["type"] == "ACTION_RESULT" && e["ref"] == "K1" && e["ok"] == false {
			found = true
			if e["code"] != "E_INVALID_TARGET" {
				t.Fatalf("code: got %v want %v", e["code"], "E_INVALID_TARGET")
			}
		}
	}
	if !found {
		t.Fatalf("expected ACTION_RESULT failure for FOLLOW")
	}
}
