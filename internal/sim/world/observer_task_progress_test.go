package world

import (
	"math"
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/tasks"
)

func TestObserverTaskProgress_MatchesAgentSemantics(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{ID: "test", Seed: 21}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "agent", DeltaVoxels: false, Out: nil, Resp: resp})
	jr := <-resp
	a := w.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	t.Run("MOVE_TO progress uses tolerance effective distance", func(t *testing.T) {
		// start at (0,0,0), target at (3,0,0), tolerance=3 => already "complete".
		a.Pos = Vec3i{X: 0, Y: 10, Z: 0}
		a.MoveTask = &tasks.MovementTask{
			TaskID:    "T1",
			Kind:      tasks.KindMoveTo,
			Target:    tasks.Vec3i{X: 3, Y: 10, Z: 0},
			Tolerance: 3,
			StartPos:  tasks.Vec3i{X: 0, Y: 10, Z: 0},
		}

		st := w.observerMoveTaskState(a, 0)
		if st == nil {
			t.Fatalf("missing move task state")
		}
		if st.Progress != 1 {
			t.Fatalf("progress=%v want 1", st.Progress)
		}
		if st.EtaTicks != 0 {
			t.Fatalf("eta=%d want 0", st.EtaTicks)
		}
	})

	t.Run("CRAFT progress uses recipe time_ticks", func(t *testing.T) {
		a.Inventory = map[string]int{}
		a.WorkTask = &tasks.WorkTask{
			TaskID:    "W1",
			Kind:      tasks.KindCraft,
			RecipeID:  "chest",
			WorkTicks: 2,
		}
		st := w.observerWorkTaskState(a)
		if st == nil {
			t.Fatalf("missing work task state")
		}
		want := 2.0 / 5.0 // chest is time_ticks=5
		if math.Abs(st.Progress-want) > 1e-6 {
			t.Fatalf("progress=%v want %v", st.Progress, want)
		}
	})
}
