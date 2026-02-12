package world

import (
	"reflect"
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/tasks"
)

func TestObsBuilderParity_SnapshotCloneProducesIdenticalOBS(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	cfg := WorldConfig{
		ID:         "obs-parity",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       99,
		BoundaryR:  4000,
	}

	w1, err := New(cfg, cats)
	if err != nil {
		t.Fatalf("world1: %v", err)
	}
	out1 := make(chan []byte, 1)
	resp1 := make(chan JoinResponse, 1)
	w1.handleJoin(JoinRequest{Name: "alpha", DeltaVoxels: true, Out: out1, Resp: resp1})
	alphaID := (<-resp1).Welcome.AgentID
	out2 := make(chan []byte, 1)
	resp2 := make(chan JoinResponse, 1)
	w1.handleJoin(JoinRequest{Name: "beta", DeltaVoxels: true, Out: out2, Resp: resp2})
	_ = (<-resp2).Welcome.AgentID

	a1 := w1.agents[alphaID]
	if a1 == nil {
		t.Fatalf("missing alpha in world1")
	}
	a1.Inventory["STONE"] = 3
	a1.Inventory["BREAD"] = 2
	a1.MoveTask = &tasks.MovementTask{
		TaskID:    "T-MOVE",
		Kind:      tasks.KindMoveTo,
		Target:    tasks.Vec3i{X: a1.Pos.X + 4, Y: 0, Z: a1.Pos.Z + 2},
		Tolerance: 1,
		StartPos:  tasks.Vec3i{X: a1.Pos.X, Y: 0, Z: a1.Pos.Z},
	}
	a1.WorkTask = &tasks.WorkTask{
		TaskID:    "T-CRAFT",
		Kind:      tasks.KindCraft,
		RecipeID:  "stick",
		Count:     1,
		WorkTicks: 1,
	}

	snapTick := uint64(7)
	snap := w1.ExportSnapshot(snapTick)
	w2, err := New(cfg, cats)
	if err != nil {
		t.Fatalf("world2: %v", err)
	}
	if err := w2.ImportSnapshot(snap); err != nil {
		t.Fatalf("import snapshot: %v", err)
	}
	a2 := w2.agents[alphaID]
	if a2 == nil {
		t.Fatalf("missing alpha in world2")
	}

	nowTick := uint64(9)
	// Normalize event/fun queues so we compare builder behavior, not prior join side effects.
	a1.Events = nil
	a2.Events = nil
	a1.EventCursor = 0
	a2.EventCursor = 0
	a1.Fun = FunScore{}
	a2.Fun = FunScore{}

	e1 := protocol.Event{"t": nowTick, "type": "CHAT", "from": "beta", "text": "hello"}
	e2 := protocol.Event{"t": nowTick, "type": "CHAT", "from": "beta", "text": "hello"}
	a1.AddEvent(e1)
	a2.AddEvent(e2)

	cl1 := &clientState{DeltaVoxels: true}
	cl2 := &clientState{DeltaVoxels: true}

	obs1 := w1.buildObs(a1, cl1, nowTick)
	obs2 := w2.buildObs(a2, cl2, nowTick)
	if !reflect.DeepEqual(obs1, obs2) {
		t.Fatalf("first OBS mismatch:\nobs1=%+v\nobs2=%+v", obs1, obs2)
	}
	if got := len(obs1.Events); got != 1 {
		t.Fatalf("expected 1 event in first OBS, got %d", got)
	}

	// Apply identical world change and ensure delta OBS also stays identical.
	stone := w1.catalogs.Blocks.Index["STONE"]
	p := Vec3i{X: a1.Pos.X + 1, Y: 0, Z: a1.Pos.Z}
	setSolid(w1, p, stone)
	setSolid(w2, p, stone)

	obs1b := w1.buildObs(a1, cl1, nowTick+1)
	obs2b := w2.buildObs(a2, cl2, nowTick+1)
	if !reflect.DeepEqual(obs1b, obs2b) {
		t.Fatalf("second OBS mismatch:\nobs1=%+v\nobs2=%+v", obs1b, obs2b)
	}
}
