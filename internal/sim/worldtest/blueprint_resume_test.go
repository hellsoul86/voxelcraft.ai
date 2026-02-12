package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestBuildBlueprint_ResumeByReissue_SkipsCorrectBlocks(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:           "test",
		TickRateHz:   5,
		DayTicks:     6000,
		ObsRadius:    7,
		Height:       1,
		Seed:         42,
		BoundaryR:    4000,
		StarterItems: map[string]int{"PLANK": 50},
	}, cats, "builder")

	self := h.LastObs().Self.Pos
	anchor := world.Vec3i{X: self[0], Y: 0, Z: self[2]}
	clearArea(t, h, anchor, 8)

	// Start building and let the first tick place some blocks.
	h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "BUILD_BLUEPRINT", BlueprintID: "road_segment", Anchor: anchor.ToArray(), Rotation: 0}}, nil)

	obs := h.LastObs()
	taskID := ""
	for _, task := range obs.Tasks {
		if task.Kind == "BUILD_BLUEPRINT" {
			taskID = task.TaskID
			break
		}
	}
	if taskID == "" {
		t.Fatalf("expected work task to start; tasks=%v", obs.Tasks)
	}

	// Cancel and re-issue the same blueprint build at the same anchor/rotation.
	h.Step(nil, nil, []string{taskID})
	obs = h.LastObs()
	for _, task := range obs.Tasks {
		if task.Kind == "BUILD_BLUEPRINT" {
			t.Fatalf("expected work task canceled; tasks=%v", obs.Tasks)
		}
	}

	h.Step(nil, []protocol.TaskReq{{ID: "K2", Type: "BUILD_BLUEPRINT", BlueprintID: "road_segment", Anchor: anchor.ToArray(), Rotation: 0}}, nil)

	// road_segment is short; advance until done.
	for i := 0; i < 5; i++ {
		h.StepNoop()
		done := true
		for _, task := range h.LastObs().Tasks {
			if task.Kind == "BUILD_BLUEPRINT" {
				done = false
				break
			}
		}
		if done {
			break
		}
	}
	for _, task := range h.LastObs().Tasks {
		if task.Kind == "BUILD_BLUEPRINT" {
			t.Fatalf("expected work task done; tasks=%v", h.LastObs().Tasks)
		}
	}
	if !h.W.CheckBlueprintPlaced("road_segment", anchor.ToArray(), 0) {
		t.Fatalf("expected blueprint to be fully placed after re-issue")
	}
}
