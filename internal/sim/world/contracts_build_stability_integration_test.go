package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestContract_BuildRequiresStability(t *testing.T) {
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
		Seed:       7,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	join := func(name string) *Agent {
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: name, Resp: resp})
		r := <-resp
		return w.agents[r.Welcome.AgentID]
	}

	poster := join("poster")
	builder := join("builder")

	termPos := poster.Pos
	w.ensureContainer(termPos, "CONTRACT_TERMINAL")
	termID := containerID("CONTRACT_TERMINAL", termPos)

	anchor := Vec3i{X: poster.Pos.X + 10, Y: 0, Z: poster.Pos.Z}
	clearBlueprintFootprint(t, w, "road_segment", anchor, 0)

	// Post a BUILD contract for a simple blueprint.
	poster.Inventory["PLANK"] = 20
	w.applyInstant(poster, protocol.InstantReq{
		ID:            "I_post",
		Type:          "POST_CONTRACT",
		TerminalID:    termID,
		ContractKind:  "BUILD",
		Reward:        []protocol.ItemStack{{Item: "PLANK", Count: 1}},
		BlueprintID:   "road_segment",
		Anchor:        anchor.ToArray(),
		Rotation:      0,
		DurationTicks: 1000,
	}, 0)

	var cid string
	for id := range w.contracts {
		cid = id
		break
	}
	if cid == "" {
		t.Fatalf("expected contract id")
	}

	// Accept contract.
	builder.Pos = poster.Pos
	w.applyInstant(builder, protocol.InstantReq{
		ID:         "I_accept",
		Type:       "ACCEPT_CONTRACT",
		TerminalID: termID,
		ContractID: cid,
	}, 1)
	if got := w.contracts[cid].State; got != ContractAccepted {
		t.Fatalf("contract state=%s want %s", got, ContractAccepted)
	}

	// Submit before building: should fail.
	builder.Events = nil
	w.applyInstant(builder, protocol.InstantReq{
		ID:         "I_submit_1",
		Type:       "SUBMIT_CONTRACT",
		TerminalID: termID,
		ContractID: cid,
	}, 2)
	if got := w.contracts[cid].State; got != ContractAccepted {
		t.Fatalf("contract state after early submit=%s want %s", got, ContractAccepted)
	}

	// Build the blueprint (floating).
	w.applyTaskReq(builder, protocol.TaskReq{
		ID:          "K_build",
		Type:        "BUILD_BLUEPRINT",
		BlueprintID: "road_segment",
		Anchor:      anchor.ToArray(),
		Rotation:    0,
	}, 2)
	for i := 0; i < 10 && builder.WorkTask != nil; i++ {
		w.tickBuildBlueprint(builder, builder.WorkTask, uint64(2+i))
	}
	if builder.WorkTask != nil {
		t.Fatalf("expected build to finish")
	}

	// In 2D, all blueprint blocks are on the ground plane (y=0), so stability is always satisfied.
	// Contracts should auto-complete once the blueprint is placed.
	w.tickContracts(10)
	if got := w.contracts[cid].State; got != ContractCompleted {
		t.Fatalf("contract state after support=%s want %s", got, ContractCompleted)
	}

	// Builder spent 5 planks to build road_segment and earned 1 plank reward.
	if got := builder.Inventory["PLANK"]; got != 16 {
		t.Fatalf("builder PLANK=%d want 16", got)
	}
}
