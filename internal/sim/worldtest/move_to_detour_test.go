package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestMoveTo_DetourWhenPrimaryAndSecondaryBlocked(t *testing.T) {
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
	target := [3]int{start.X + 5, 0, start.Z}

	// Make primary (+X), secondary (+Z), and back (-X) all blocked, but leave a detour via -Z.
	h.SetBlock(start, "AIR")
	h.SetBlock(world.Vec3i{X: start.X + 1, Y: 0, Z: start.Z}, "STONE") // +X
	h.SetBlock(world.Vec3i{X: start.X, Y: 0, Z: start.Z + 1}, "STONE") // +Z
	h.SetBlock(world.Vec3i{X: start.X - 1, Y: 0, Z: start.Z}, "STONE") // -X

	// Clear a corridor along -Z then +X so a detour exists within depth 16.
	for x := start.X; x <= start.X+5; x++ {
		h.SetBlock(world.Vec3i{X: x, Y: 0, Z: start.Z - 1}, "AIR")
	}

	h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "MOVE_TO", Target: target, Tolerance: 1.2}}, nil)
	obs := h.LastObs()

	if got := obs.Self.Pos; got != ([3]int{start.X, 0, start.Z - 1}) {
		t.Fatalf("pos=%+v want detour step %+v", got, [3]int{start.X, 0, start.Z - 1})
	}
}
