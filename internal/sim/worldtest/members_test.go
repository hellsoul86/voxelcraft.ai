package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestLandMembersPermissions(t *testing.T) {
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
		Seed:       1,
		BoundaryR:  4000,
		StarterItems: map[string]int{
			"BATTERY":      5,
			"CRYSTAL_SHARD": 5,
			"CHEST":        1,
			"PLANK":        20,
		},
	}, cats, "owner")
	owner := h.DefaultAgentID
	member := h.Join("member")

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")

	// Claim land. Visitors have no build/break/trade.
	h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:     "K_claim",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 32,
	}}, nil)
	if got := actionResultCode(h.LastObsFor(owner), "K_claim"); got != "" {
		t.Fatalf("claim expected ok, got code=%q", got)
	}
	landID := actionResultFieldString(h.LastObsFor(owner), "K_claim", "land_id")
	if landID == "" {
		t.Fatalf("expected land_id in claim ACTION_RESULT")
	}

	// Put member inside the land.
	h.SetAgentPosFor(member, anchor)

	// As visitor, MARKET chat should be denied inside the claim (allow_trade=false).
	h.ClearAgentEventsFor(member)
	h.StepFor(member, []protocol.InstantReq{{
		ID:      "I_mkt",
		Type:    "SAY",
		Channel: "MARKET",
		Text:    "hi",
	}}, nil, nil)
	if got := actionResultCode(h.LastObsFor(member), "I_mkt"); got != "E_NO_PERMISSION" {
		t.Fatalf("visitor market say code=%q want %q", got, "E_NO_PERMISSION")
	}

	// As visitor, PLACE should be denied.
	placePos := world.Vec3i{X: anchor.X + 2, Y: 0, Z: anchor.Z}
	h.SetBlock(placePos, "AIR")
	h.ClearAgentEventsFor(member)
	h.StepFor(member, nil, []protocol.TaskReq{{
		ID:       "K_place",
		Type:     "PLACE",
		ItemID:   "CHEST",
		BlockPos: placePos.ToArray(),
	}}, nil)
	if !hasTaskFail(h.LastObsFor(member), "E_NO_PERMISSION") {
		t.Fatalf("expected PLACE to fail with E_NO_PERMISSION; events=%v", h.LastObsFor(member).Events)
	}

	// Add member via instant.
	h.ClearAgentEventsFor(owner)
	h.StepFor(owner, []protocol.InstantReq{{
		ID:       "I_add",
		Type:     "ADD_MEMBER",
		LandID:   landID,
		MemberID: member,
	}}, nil, nil)
	if got := actionResultCode(h.LastObsFor(owner), "I_add"); got != "" {
		t.Fatalf("add member expected ok, got code=%q", got)
	}

	// Member can now PLACE.
	h.ClearAgentEventsFor(member)
	h.StepFor(member, nil, []protocol.TaskReq{{
		ID:       "K_place2",
		Type:     "PLACE",
		ItemID:   "CHEST",
		BlockPos: placePos.ToArray(),
	}}, nil)
	if hasTaskFail(h.LastObsFor(member), "") {
		t.Fatalf("expected PLACE to succeed; events=%v", h.LastObsFor(member).Events)
	}

	chestID := findEntityIDAt(h.LastObsFor(member), "CHEST", placePos.ToArray())
	if chestID == "" {
		t.Fatalf("expected CHEST entity at %v; entities=%v", placePos, h.LastObsFor(member).Entities)
	}

	// Deposit 5 PLANK into chest as owner (deposit doesn't require withdraw permission).
	h.SetAgentPosFor(owner, placePos)
	h.AddInventoryFor(owner, "PLANK", 5)
	h.ClearAgentEventsFor(owner)
	h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:    "K_dep",
		Type:  "TRANSFER",
		Src:   "SELF",
		Dst:   chestID,
		ItemID: "PLANK",
		Count: 5,
	}}, nil)
	if hasTaskFail(h.LastObsFor(owner), "") {
		t.Fatalf("expected deposit to succeed; events=%v", h.LastObsFor(owner).Events)
	}

	// Clear member PLANK count so we can assert exact transfer.
	h.AddInventoryFor(member, "PLANK", -9999)
	h.SetAgentPosFor(member, placePos)
	h.ClearAgentEventsFor(member)
	h.StepFor(member, nil, []protocol.TaskReq{{
		ID:    "K_wd",
		Type:  "TRANSFER",
		Src:   chestID,
		Dst:   "SELF",
		ItemID: "PLANK",
		Count: 5,
	}}, nil)
	if hasTaskFail(h.LastObsFor(member), "") {
		t.Fatalf("expected withdraw to succeed for member; events=%v", h.LastObsFor(member).Events)
	}
	if got := invCount(h.LastObsFor(member).Inventory, "PLANK"); got != 5 {
		t.Fatalf("member PLANK after withdraw: got %d want %d", got, 5)
	}

	// Remove member.
	h.ClearAgentEventsFor(owner)
	h.StepFor(owner, []protocol.InstantReq{{
		ID:       "I_remove",
		Type:     "REMOVE_MEMBER",
		LandID:   landID,
		MemberID: member,
	}}, nil, nil)
	if got := actionResultCode(h.LastObsFor(owner), "I_remove"); got != "" {
		t.Fatalf("remove member expected ok, got code=%q", got)
	}

	// Withdraw should now be denied.
	h.ClearAgentEventsFor(member)
	h.StepFor(member, nil, []protocol.TaskReq{{
		ID:    "K_wd2",
		Type:  "TRANSFER",
		Src:   chestID,
		Dst:   "SELF",
		ItemID: "PLANK",
		Count: 1,
	}}, nil)
	if !hasTaskFail(h.LastObsFor(member), "E_NO_PERMISSION") {
		t.Fatalf("expected withdraw denied after removal; events=%v", h.LastObsFor(member).Events)
	}
}
