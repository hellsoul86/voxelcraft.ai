package runtime

import (
	"testing"

	"voxelcraft.ai/internal/sim/tasks"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func TestBuildMoveTaskStateFromWorldNilSafe(t *testing.T) {
	if st := BuildMoveTaskStateFromWorld(nil, nil); st != nil {
		t.Fatalf("expected nil for nil agent")
	}
}

func TestBuildWorkTaskStateFromWorld(t *testing.T) {
	a := &modelpkg.Agent{
		ID: "A1",
		WorkTask: &tasks.WorkTask{
			TaskID: "T1",
			Kind:   tasks.KindCraft,
		},
	}
	st := BuildWorkTaskStateFromWorld(a, 0.4)
	if st == nil {
		t.Fatalf("expected task state")
	}
	if st.Kind != string(tasks.KindCraft) {
		t.Fatalf("unexpected kind: %s", st.Kind)
	}
	if st.Progress != 0.4 {
		t.Fatalf("unexpected progress: %v", st.Progress)
	}
}
