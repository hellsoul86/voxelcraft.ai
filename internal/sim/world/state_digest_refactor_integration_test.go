package world

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/tasks"
)

func TestStateDigestRefactorParity_SnapshotAndTicks(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	cfg := WorldConfig{
		ID:         "digest-refactor",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
	}

	w1, err := New(cfg, cats)
	if err != nil {
		t.Fatalf("world1: %v", err)
	}

	join := func(name string) string {
		out := make(chan []byte, 1)
		resp := make(chan JoinResponse, 1)
		w1.handleJoin(JoinRequest{Name: name, DeltaVoxels: true, Out: out, Resp: resp})
		return (<-resp).Welcome.AgentID
	}

	a1ID := join("alpha")
	a2ID := join("beta")
	a1 := w1.agents[a1ID]
	a2 := w1.agents[a2ID]
	if a1 == nil || a2 == nil {
		t.Fatalf("missing joined agents")
	}

	// Build a non-trivial deterministic state so digest ordering matters.
	a1.Inventory["PLANK"] = 7
	a1.Inventory["COAL"] = 3
	a2.Inventory["IRON_INGOT"] = 2
	a2.Inventory["BREAD"] = 4
	a1.MoveTask = &tasks.MovementTask{
		TaskID:    "T-MOVE",
		Kind:      tasks.KindMoveTo,
		Target:    tasks.Vec3i{X: a1.Pos.X + 5, Y: 0, Z: a1.Pos.Z + 1},
		Tolerance: 1,
		StartPos:  tasks.Vec3i{X: a1.Pos.X, Y: 0, Z: a1.Pos.Z},
	}
	a2.WorkTask = &tasks.WorkTask{
		TaskID:    "T-CRAFT",
		Kind:      tasks.KindCraft,
		RecipeID:  "stick",
		Count:     1,
		WorkTicks: 2,
	}
	stone := w1.catalogs.Blocks.Index["STONE"]
	setSolid(w1, Vec3i{X: a1.Pos.X + 1, Y: 0, Z: a1.Pos.Z}, stone)
	setSolid(w1, Vec3i{X: a2.Pos.X - 1, Y: 0, Z: a2.Pos.Z}, stone)

	for i := 0; i < 6; i++ {
		w1.step(nil, nil, nil)
	}
	snapTick := w1.CurrentTick() - 1
	d1 := w1.stateDigest(snapTick)
	snap := w1.ExportSnapshot(snapTick)

	w2, err := New(cfg, cats)
	if err != nil {
		t.Fatalf("world2: %v", err)
	}
	if err := w2.ImportSnapshot(snap); err != nil {
		t.Fatalf("import snapshot: %v", err)
	}
	d2 := w2.stateDigest(snapTick)
	if d1 != d2 {
		t.Fatalf("digest mismatch after import: %s vs %s", d1, d2)
	}

	// Additional parity guard: advance both worlds and assert digest stays aligned.
	for i := 0; i < 5; i++ {
		t1, _ := w1.StepOnce(nil, nil, nil)
		t2, _ := w2.StepOnce(nil, nil, nil)
		if t1 != t2 {
			t.Fatalf("tick mismatch: %d vs %d", t1, t2)
		}
		x1 := w1.stateDigest(t1)
		x2 := w2.stateDigest(t2)
		if x1 != x2 {
			t.Fatalf("digest mismatch at tick %d: %s vs %s", t1, x1, x2)
		}
	}
}
