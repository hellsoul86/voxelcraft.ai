package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestMoveTo_OutOfBoundsRejectedAtStart(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{ID: "test", Seed: 41, BoundaryR: 8, Height: 1, TickRateHz: 5, DayTicks: 6000, ObsRadius: 7}, cats, "mover")

	// Advance to tick 1 (some validations gate on tick boundaries).
	h.StepNoop()

	h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "MOVE_TO", Target: [3]int{9, 0, 0}, Tolerance: 1.2}}, nil)
	obs := h.LastObs()

	// Ensure task did not start.
	for _, task := range obs.Tasks {
		if task.Kind == "MOVE_TO" {
			t.Fatalf("expected no move task for out-of-bounds target")
		}
	}

	found := false
	for _, e := range obs.Events {
		if e["type"] != "ACTION_RESULT" || e["ref"] != "K1" {
			continue
		}
		found = true
		if ok, _ := e["ok"].(bool); ok {
			t.Fatalf("expected ok=false for out-of-bounds MOVE_TO")
		}
		if code, _ := e["code"].(string); code != "E_INVALID_TARGET" {
			t.Fatalf("expected E_INVALID_TARGET, got %v", e["code"])
		}
		break
	}
	if !found {
		t.Fatalf("expected ACTION_RESULT for rejected MOVE_TO; events=%v", obs.Events)
	}
}
