package worldtest

import (
	"fmt"
	"testing"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func findContractV1(s snapshot.SnapshotV1, contractID string) *snapshot.ContractV1 {
	for i := range s.Contracts {
		c := &s.Contracts[i]
		if c.ContractID == contractID {
			return c
		}
	}
	return nil
}

func TestContract_BuildRequiresStability(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	h := NewHarness(t, world.WorldConfig{
		ID:         "test",
		WorldType:  "OVERWORLD",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       7,
		BoundaryR:  4000,
		StarterItems: map[string]int{}, // deterministic inventories for assertions
	}, cats, "poster")
	poster := h.DefaultAgentID
	builder := h.Join("builder")

	selfArr := h.LastObsFor(poster).Self.Pos
	posterPos := world.Vec3i{X: selfArr[0], Y: 0, Z: selfArr[2]}
	clearArea(t, h, posterPos, 12)

	// Place a contract terminal at poster position.
	h.SetBlock(posterPos, "AIR")
	h.AddInventoryFor(poster, "CONTRACT_TERMINAL", 1)
	h.ClearAgentEventsFor(poster)
	obs := h.StepFor(poster, nil, []protocol.TaskReq{{
		ID:       "K_place_term",
		Type:     "PLACE",
		ItemID:   "CONTRACT_TERMINAL",
		BlockPos: posterPos.ToArray(),
	}}, nil)
	if hasTaskFail(obs, "") {
		t.Fatalf("place terminal failed: events=%v", obs.Events)
	}
	termID := fmt.Sprintf("CONTRACT_TERMINAL@%d,%d,%d", posterPos.X, posterPos.Y, posterPos.Z)

	// Post a BUILD contract for road_segment blueprint.
	anchor := world.Vec3i{X: posterPos.X + 10, Y: 0, Z: posterPos.Z}
	clearArea(t, h, anchor, 8)

	h.AddInventoryFor(poster, "PLANK", 20)
	h.ClearAgentEventsFor(poster)
	obs = h.StepFor(poster, []protocol.InstantReq{{
		ID:            "I_post",
		Type:          "POST_CONTRACT",
		TerminalID:    termID,
		ContractKind:  "BUILD",
		Reward:        []protocol.ItemStack{{Item: "PLANK", Count: 1}},
		BlueprintID:   "road_segment",
		Anchor:        anchor.ToArray(),
		Rotation:      0,
		DurationTicks: 1000,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_post"); got != "" {
		t.Fatalf("POST_CONTRACT expected ok, got code=%q events=%v", got, obs.Events)
	}
	contractID := actionResultFieldString(obs, "I_post", "contract_id")
	if contractID == "" {
		t.Fatalf("missing contract_id; events=%v", obs.Events)
	}

	// Builder accepts and tries to submit before building: should fail.
	h.SetAgentPosFor(builder, posterPos)
	h.AddInventoryFor(builder, "PLANK", 20)
	h.StepNoop()

	h.ClearAgentEventsFor(builder)
	obs = h.StepFor(builder, []protocol.InstantReq{{
		ID:         "I_accept",
		Type:       "ACCEPT_CONTRACT",
		TerminalID: termID,
		ContractID: contractID,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_accept"); got != "" {
		t.Fatalf("ACCEPT_CONTRACT expected ok, got code=%q events=%v", got, obs.Events)
	}

	h.ClearAgentEventsFor(builder)
	obs = h.StepFor(builder, []protocol.InstantReq{{
		ID:         "I_submit_early",
		Type:       "SUBMIT_CONTRACT",
		TerminalID: termID,
		ContractID: contractID,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_submit_early"); got != "E_BLOCKED" {
		t.Fatalf("SUBMIT_CONTRACT early expected E_BLOCKED, got code=%q events=%v", got, obs.Events)
	}

	// Build the blueprint; in 2D it is always stable and contract should auto-complete.
	h.ClearAgentEventsFor(builder)
	obs = h.StepFor(builder, nil, []protocol.TaskReq{{
		ID:          "K_build",
		Type:        "BUILD_BLUEPRINT",
		BlueprintID: "road_segment",
		Anchor:      anchor.ToArray(),
		Rotation:    0,
	}}, nil)
	if got := actionResultCode(obs, "K_build"); got != "" {
		t.Fatalf("BUILD_BLUEPRINT expected ok, got code=%q events=%v", got, obs.Events)
	}
	for i := 0; i < 20; i++ {
		done := true
		for _, task := range h.LastObsFor(builder).Tasks {
			if task.Kind == "BUILD_BLUEPRINT" {
				done = false
				break
			}
		}
		if done {
			break
		}
		h.StepNoop()
	}
	for _, task := range h.LastObsFor(builder).Tasks {
		if task.Kind == "BUILD_BLUEPRINT" {
			t.Fatalf("expected build task done; tasks=%v", h.LastObsFor(builder).Tasks)
		}
	}

	_, snap := h.Snapshot()
	c := findContractV1(snap, contractID)
	if c == nil {
		t.Fatalf("missing contract %q in snapshot", contractID)
	}
	if c.State != "COMPLETED" {
		t.Fatalf("contract state=%q want %q", c.State, "COMPLETED")
	}

	// Builder spent 5 planks to build road_segment and earned 1 plank reward.
	if got := invCount(h.LastObsFor(builder).Inventory, "PLANK"); got != 16 {
		t.Fatalf("builder PLANK=%d want 16", got)
	}
}
