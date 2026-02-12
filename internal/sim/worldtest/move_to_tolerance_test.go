package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestMoveTo_ToleranceCompletesWithinRadius_NoTeleport(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
	}, cats, "bot")

	start := h.LastObs().Self.Pos
	target := [3]int{start[0] + 2, start[1], start[2]}

	h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "MOVE_TO", Target: target, Tolerance: 3}}, nil)

	// Within tolerance, the move completes immediately and position does not change.
	obs := h.LastObs()
	for _, task := range obs.Tasks {
		if task.Kind == "MOVE_TO" {
			t.Fatalf("expected move task to complete immediately within tolerance")
		}
	}
	if got := obs.Self.Pos; got != start {
		t.Fatalf("expected move task to complete immediately within tolerance")
	}
	found := false
	for _, ev := range obs.Events {
		if ev["type"] == "TASK_DONE" && ev["kind"] == "MOVE_TO" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected TASK_DONE event; events=%v", obs.Events)
	}
}

func TestMoveTo_PrimaryAxisBlocked_TriesSecondaryAxis(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
	}, cats, "bot")

	start := world.Vec3i{X: 10, Y: 0, Z: 10}
	h.SetAgentPos(start)
	target := [3]int{start.X + 5, start.Y, start.Z + 1} // abs(dx)>abs(dz) => primary axis X

	pillar := world.Vec3i{X: start.X + 1, Y: 0, Z: start.Z}
	secondary := world.Vec3i{X: start.X, Y: 0, Z: start.Z + 1}
	h.SetBlock(start, "AIR")
	h.SetBlock(secondary, "AIR") // secondary axis cell must be passable
	h.SetBlock(pillar, "STONE")  // primary axis blocked

	h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "MOVE_TO", Target: target, Tolerance: 1.2}}, nil)

	// Primary (+X) is blocked; expect one step along +Z.
	obs := h.LastObs()
	if gotX, gotZ := obs.Self.Pos[0], obs.Self.Pos[2]; gotX != start.X || gotZ != start.Z+1 {
		t.Fatalf("pos=(%d,%d) want=(%d,%d)", gotX, gotZ, start.X, start.Z+1)
	}
}
