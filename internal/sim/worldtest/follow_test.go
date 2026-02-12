package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestFollow_MovesTowardTargetAndMaintainsDistance(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{ID: "test", Seed: 42}, cats, "leader")
	leader := h.DefaultAgentID
	follower := h.Join("follower")

	h.SetAgentPosFor(leader, world.Vec3i{X: 0, Y: 0, Z: 0})
	h.SetAgentPosFor(follower, world.Vec3i{X: 10, Y: 0, Z: 0})

	// Clear a corridor so movement isn't blocked by generated solids.
	for x := 0; x <= 10; x++ {
		h.SetBlock(world.Vec3i{X: x, Y: 0, Z: 0}, "AIR")
	}

	h.StepFor(follower, nil, []protocol.TaskReq{{
		ID:       "K_follow",
		Type:     "FOLLOW",
		TargetID: leader,
		Distance: 1.0,
	}}, nil)
	if got := actionResultCode(h.LastObsFor(follower), "K_follow"); got != "" {
		t.Fatalf("FOLLOW start expected ok, got code=%q events=%v", got, h.LastObsFor(follower).Events)
	}

	// Advance until follower should be within distance 1.
	for i := 0; i < 8; i++ {
		h.StepNoop()
	}
	if got := h.LastObsFor(follower).Self.Pos[0]; got != 1 {
		t.Fatalf("follower x: got %d want %d", got, 1)
	}

	// One more tick should not move closer than distance=1.
	h.StepNoop()
	if got := h.LastObsFor(follower).Self.Pos[0]; got != 1 {
		t.Fatalf("follower should hold distance: got x=%d want %d", got, 1)
	}
	if !hasMoveTaskKind(h.LastObsFor(follower), "FOLLOW") {
		t.Fatalf("expected FOLLOW move task to remain active; tasks=%v", h.LastObsFor(follower).Tasks)
	}

	// Move leader away; follower should start moving again.
	h.SetAgentPosFor(leader, world.Vec3i{X: 5, Y: 0, Z: 0})
	h.StepNoop()
	if got := h.LastObsFor(follower).Self.Pos[0]; got != 2 {
		t.Fatalf("follower should advance after leader moved: got x=%d want %d", got, 2)
	}
}

func TestFollow_InvalidTargetFails(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{ID: "test", Seed: 42}, cats, "follower")

	obs := h.Step(nil, []protocol.TaskReq{{
		ID:       "K_follow",
		Type:     "FOLLOW",
		TargetID: "NOPE",
		Distance: 2.0,
	}}, nil)
	if got := actionResultCode(obs, "K_follow"); got != "E_INVALID_TARGET" {
		t.Fatalf("expected E_INVALID_TARGET, got code=%q events=%v", got, obs.Events)
	}
	if hasMoveTaskKind(obs, "FOLLOW") {
		t.Fatalf("expected no FOLLOW task to be active; tasks=%v", obs.Tasks)
	}
}

func hasMoveTaskKind(obs protocol.ObsMsg, want string) bool {
	for _, t := range obs.Tasks {
		if t.Kind == want {
			return true
		}
	}
	return false
}

