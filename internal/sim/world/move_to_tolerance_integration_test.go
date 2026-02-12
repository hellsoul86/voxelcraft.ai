package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestMoveTo_ToleranceCompletesWithinRadius_NoTeleport(t *testing.T) {
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

	start := a.Pos
	target := Vec3i{X: start.X + 2, Y: start.Y, Z: start.Z}

	a.Events = nil
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "MOVE_TO", Target: target.ToArray(), Tolerance: 3}},
	}}})

	if a.MoveTask != nil {
		t.Fatalf("expected move task to complete immediately within tolerance")
	}
	if got := a.Pos; got != start {
		t.Fatalf("expected no teleport within tolerance; pos=%+v want %+v", got, start)
	}
	foundDone := false
	for _, ev := range a.Events {
		if ev["type"] == "TASK_DONE" && ev["kind"] == "MOVE_TO" {
			foundDone = true
			break
		}
	}
	if !foundDone {
		t.Fatalf("expected TASK_DONE event; events=%v", a.Events)
	}
}

func TestMoveTo_PrimaryAxisBlocked_TriesSecondaryAxis(t *testing.T) {
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

	// Place the agent at a deterministic surface location.
	start := Vec3i{X: 10, Y: 0, Z: 10}
	a.Pos = start
	target := Vec3i{X: start.X + 5, Y: start.Y, Z: start.Z + 1} // abs(dx)>abs(dz) => primary axis X

	stone := cats.Blocks.Index["STONE"]
	pillarX := start.X + 1
	pillarZ := start.Z
	setAir(w, start)
	setAir(w, Vec3i{X: start.X, Y: 0, Z: start.Z + 1}) // secondary axis cell must be passable
	setSolid(w, Vec3i{X: pillarX, Y: 0, Z: pillarZ}, stone)

	a.Events = nil
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "MOVE_TO", Target: target.ToArray(), Tolerance: 1.2}},
	}}})

	// Primary (+X) is blocked; expect one step along +Z.
	if gotX, gotZ := a.Pos.X, a.Pos.Z; gotX != start.X || gotZ != start.Z+1 {
		t.Fatalf("pos=(%d,%d) want=(%d,%d)", gotX, gotZ, start.X, start.Z+1)
	}
}
