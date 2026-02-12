package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestBulletinBoard_Physical_PostOpenAndPermissions(t *testing.T) {
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
		Seed:       42,
		BoundaryR:  4000,
		StarterItems: map[string]int{
			"BATTERY":        5,
			"CRYSTAL_SHARD":  5,
			"BULLETIN_BOARD": 2,
		},
		RateLimits: world.RateLimitConfig{
			PostBoardWindowTicks: 1,
			PostBoardMax:         10,
		},
	}, cats, "owner")
	owner := h.DefaultAgentID
	visitor := h.Join("visitor")

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")

	// Claim a homestead: visitors cannot trade/post by default.
	h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:     "K_claim",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 32,
	}}, nil)
	if got := actionResultCode(h.LastObsFor(owner), "K_claim"); got != "" {
		t.Fatalf("claim expected ok, got code=%q", got)
	}

	// Place a bulletin board inside the claim.
	boardPos := world.Vec3i{X: anchor.X + 1, Y: 0, Z: anchor.Z}
	h.SetBlock(boardPos, "AIR")

	h.ClearAgentEventsFor(owner)
	h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:       "K_place",
		Type:     "PLACE",
		ItemID:   "BULLETIN_BOARD",
		BlockPos: boardPos.ToArray(),
	}}, nil)
	if got := actionResultCode(h.LastObsFor(owner), "K_place"); got != "" {
		t.Fatalf("place expected ok, got code=%q", got)
	}

	boardID := findEntityIDAt(h.LastObsFor(owner), "BULLETIN_BOARD", boardPos.ToArray())
	if boardID == "" {
		t.Fatalf("missing BULLETIN_BOARD entity at %v; entities=%v", boardPos, h.LastObsFor(owner).Entities)
	}

	// Visitor is nearby but not a member: cannot post when allow_trade=false.
	h.SetAgentPosFor(visitor, boardPos)
	h.ClearAgentEventsFor(visitor)
	h.StepFor(visitor, []protocol.InstantReq{{
		ID:       "I_post_v",
		Type:     "POST_BOARD",
		TargetID: boardID,
		Title:    "hello",
		Body:     "world",
	}}, nil, nil)
	if got := actionResultCode(h.LastObsFor(visitor), "I_post_v"); got != "E_NO_PERMISSION" {
		t.Fatalf("visitor post result code=%q want %q", got, "E_NO_PERMISSION")
	}

	// Owner can post.
	h.ClearAgentEventsFor(owner)
	h.StepFor(owner, []protocol.InstantReq{{
		ID:       "I_post_o",
		Type:     "POST_BOARD",
		TargetID: boardID,
		Title:    "notice",
		Body:     "market day",
	}}, nil, nil)
	if got := actionResultCode(h.LastObsFor(owner), "I_post_o"); got != "" {
		t.Fatalf("owner post expected ok, got code=%q", got)
	}

	// OPEN should return board contents to anyone in range.
	h.ClearAgentEventsFor(visitor)
	h.StepFor(visitor, nil, []protocol.TaskReq{{
		ID:       "K_open",
		Type:     "OPEN",
		TargetID: boardID,
	}}, nil)

	found := false
	for _, ev := range h.LastObsFor(visitor).Events {
		if ev["type"] == "BOARD" && ev["board_id"] == boardID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected BOARD event from OPEN; events=%v", h.LastObsFor(visitor).Events)
	}

	// Too far: posting should fail with E_BLOCKED.
	h.SetAgentPosFor(owner, world.Vec3i{X: boardPos.X + 10, Y: 0, Z: boardPos.Z})
	h.ClearAgentEventsFor(owner)
	h.StepFor(owner, []protocol.InstantReq{{
		ID:       "I_post_far_o",
		Type:     "POST_BOARD",
		TargetID: boardID,
		Title:    "x",
		Body:     "y",
	}}, nil, nil)
	if got := actionResultCode(h.LastObsFor(owner), "I_post_far_o"); got != "E_BLOCKED" {
		t.Fatalf("expected E_BLOCKED when too far, got code=%q", got)
	}
}

