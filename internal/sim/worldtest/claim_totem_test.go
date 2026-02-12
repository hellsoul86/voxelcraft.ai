package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestClaimTotem_MiningRemovesClaimAndBoundLaws(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:           "test",
		WorldType:    "OVERWORLD",
		Seed:         4,
		LawNoticeTicks: 1,
		LawVoteTicks:   1,
	}, cats, "owner")
	owner := h.DefaultAgentID

	h.AddInventoryFor(owner, "BATTERY", 3)
	h.AddInventoryFor(owner, "CRYSTAL_SHARD", 3)
	h.AddInventoryFor(owner, "IRON_PICKAXE", 1)

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")

	// Claim land.
	h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:     "K_claim",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 32,
	}}, nil)
	obs := h.LastObsFor(owner)
	if got := actionResultCode(obs, "K_claim"); got != "" {
		t.Fatalf("claim expected ok, got code=%q events=%v", got, obs.Events)
	}
	landID := actionResultFieldString(obs, "K_claim", "land_id")
	if landID == "" {
		t.Fatalf("missing land_id; events=%v", obs.Events)
	}

	// Propose and activate market tax law bound to the claim.
	h.ClearAgentEventsFor(owner)
	obs = h.StepFor(owner, []protocol.InstantReq{{
		ID:         "I_prop",
		Type:       "PROPOSE_LAW",
		LandID:     landID,
		TemplateID: "MARKET_TAX",
		Title:      "t",
		Params:     map[string]interface{}{"market_tax": 0.05},
	}}, nil, nil)
	if got := actionResultCode(obs, "I_prop"); got != "" {
		t.Fatalf("propose expected ok, got code=%q events=%v", got, obs.Events)
	}
	lawID := actionResultFieldString(obs, "I_prop", "law_id")
	if lawID == "" {
		t.Fatalf("missing law_id; events=%v", obs.Events)
	}

	// Notice -> voting.
	h.StepNoop()

	h.ClearAgentEventsFor(owner)
	obs = h.StepFor(owner, []protocol.InstantReq{{
		ID:     "I_vote",
		Type:   "VOTE",
		LawID:  lawID,
		Choice: "YES",
	}}, nil, nil)
	if got := actionResultCode(obs, "I_vote"); got != "" {
		t.Fatalf("vote expected ok, got code=%q events=%v", got, obs.Events)
	}
	// Ensure the tax is applied.
	if got := obs.LocalRules.Tax["market"]; got < 0.049 || got > 0.051 {
		t.Fatalf("expected market tax ~0.05 after activation, got %v", got)
	}

	// Mine the claim totem block at anchor.
	h.ClearAgentEventsFor(owner)
	h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:       "K_mine",
		Type:     "MINE",
		BlockPos: anchor.ToArray(),
	}}, nil)
	obs = stepUntilWorkDoneFor(t, h, owner, "MINE", 100)

	// Claim removed: local rules should be wild at the anchor.
	if obs.LocalRules.LandID != "" {
		t.Fatalf("expected claim removed (land_id empty), got %q", obs.LocalRules.LandID)
	}
	if got := obs.LocalRules.Tax["market"]; got != 0.0 {
		t.Fatalf("expected market tax reset after claim removal, got %v", got)
	}

	// Bound laws should be removed: voting again should fail with E_INVALID_TARGET.
	h.ClearAgentEventsFor(owner)
	obs = h.StepFor(owner, []protocol.InstantReq{{
		ID:     "I_vote2",
		Type:   "VOTE",
		LawID:  lawID,
		Choice: "YES",
	}}, nil, nil)
	if got := actionResultCode(obs, "I_vote2"); got != "E_INVALID_TARGET" {
		t.Fatalf("expected E_INVALID_TARGET after law removal, got code=%q events=%v", got, obs.Events)
	}

	// Ensure re-claim works after totem removal.
	h.ClearAgentEventsFor(owner)
	obs = h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:     "K_reclaim",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 32,
	}}, nil)
	if got := actionResultCode(obs, "K_reclaim"); got != "" {
		t.Fatalf("expected re-claim to succeed, got code=%q events=%v", got, obs.Events)
	}
}
