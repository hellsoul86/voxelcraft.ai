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
		Height:     64,
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

	anchor := Vec3i{X: poster.Pos.X + 10, Y: poster.Pos.Y + 5, Z: poster.Pos.Z}
	if got := w.chunks.GetBlock(anchor); got != w.chunks.gen.Air {
		t.Fatalf("expected build anchor in air; got block=%q", w.blockName(got))
	}

	// Post a BUILD contract for a simple blueprint at a floating anchor (initially unstable).
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

	// Not stable: should not auto-complete and submit should fail.
	w.tickContracts(10)
	if got := w.contracts[cid].State; got != ContractAccepted {
		t.Fatalf("contract state after unstable build=%s want %s", got, ContractAccepted)
	}
	builder.Events = nil
	w.applyInstant(builder, protocol.InstantReq{
		ID:         "I_submit_1",
		Type:       "SUBMIT_CONTRACT",
		TerminalID: termID,
		ContractID: cid,
	}, 10)
	if got := w.contracts[cid].State; got != ContractAccepted {
		t.Fatalf("contract state after unstable submit=%s want %s", got, ContractAccepted)
	}
	foundBlocked := false
	for _, ev := range builder.Events {
		if ev["type"] != "ACTION_RESULT" || ev["ref"] != "I_submit_1" {
			continue
		}
		if ok, _ := ev["ok"].(bool); ok {
			continue
		}
		if ev["code"] == "E_BLOCKED" {
			foundBlocked = true
		}
	}
	if !foundBlocked {
		t.Fatalf("expected E_BLOCKED action result; events=%v", builder.Events)
	}

	// Add a support block under the structure so stability passes.
	stoneID, ok := w.catalogs.Blocks.Index["STONE"]
	if !ok {
		t.Fatalf("missing STONE block id")
	}
	w.chunks.SetBlock(Vec3i{X: anchor.X, Y: anchor.Y - 1, Z: anchor.Z}, stoneID)

	w.tickContracts(11)
	if got := w.contracts[cid].State; got != ContractCompleted {
		t.Fatalf("contract state after support=%s want %s", got, ContractCompleted)
	}

	// Builder spent 5 planks to build road_segment and earned 1 plank reward.
	if got := builder.Inventory["PLANK"]; got != 16 {
		t.Fatalf("builder PLANK=%d want 16", got)
	}
}
