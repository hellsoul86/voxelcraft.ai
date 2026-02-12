package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestLaw_FineBreakPerBlock_FinesDeniedMine(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:            "test",
		WorldType:     "OVERWORLD",
		TickRateHz:    5,
		DayTicks:      6000,
		ObsRadius:     7,
		Height:        1,
		Seed:          42,
		BoundaryR:     4000,
		LawNoticeTicks: 1,
		LawVoteTicks:   1,
		StarterItems: map[string]int{
			"BATTERY":      5,
			"CRYSTAL_SHARD": 5,
		},
	}, cats, "owner")
	owner := h.DefaultAgentID
	visitor := h.Join("visitor")

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")

	// Ensure predictable balances.
	h.AddInventoryFor(owner, "IRON_INGOT", -9999)
	h.AddInventoryFor(visitor, "IRON_INGOT", 10)

	// Claim land.
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

	// Activate fine law (PROPOSE -> VOTE yes).
	lawID := proposeLaw(t, h, owner, landID, "FINE_BREAK_PER_BLOCK", map[string]interface{}{
		"fine_item":      "IRON_INGOT",
		"fine_per_block": 3,
	})
	voteYesAndActivate(t, h, owner, lawID)

	// Place visitor inside the land and attempt to mine a block: should be denied and fined.
	h.SetAgentPosFor(visitor, anchor)
	pos := world.Vec3i{X: anchor.X + 1, Y: 0, Z: anchor.Z}
	h.SetBlock(pos, "STONE")

	h.ClearAgentEventsFor(visitor)
	h.StepFor(visitor, nil, []protocol.TaskReq{{
		ID:       "K_mine",
		Type:     "MINE",
		BlockPos: [3]int{pos.X, 0, pos.Z},
	}}, nil)

	obsV := h.LastObsFor(visitor)
	obsO := h.LastObsFor(owner)

	if got := invCount(obsV.Inventory, "IRON_INGOT"); got != 7 {
		t.Fatalf("visitor fine: got %d want %d", got, 7)
	}
	if got := invCount(obsO.Inventory, "IRON_INGOT"); got != 3 {
		t.Fatalf("owner fine credit: got %d want %d", got, 3)
	}
	b, err := h.W.DebugGetBlock(pos)
	if err != nil {
		t.Fatalf("DebugGetBlock: %v", err)
	}
	if gotName := h.Cats.Blocks.Palette[b]; gotName != "STONE" {
		t.Fatalf("block should remain (denied mine): got %q want %q", gotName, "STONE")
	}
}

func TestLaw_AccessPassCore_ChargesOnCoreEntry(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:            "test",
		WorldType:     "OVERWORLD",
		TickRateHz:    5,
		DayTicks:      6000,
		ObsRadius:     7,
		Height:        1,
		Seed:          42,
		BoundaryR:     4000,
		LawNoticeTicks: 1,
		LawVoteTicks:   1,
		StarterItems: map[string]int{
			"BATTERY":      5,
			"CRYSTAL_SHARD": 5,
		},
	}, cats, "owner")
	owner := h.DefaultAgentID
	visitor := h.Join("visitor")

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")

	h.AddInventoryFor(owner, "IRON_INGOT", -9999)
	h.AddInventoryFor(visitor, "IRON_INGOT", 5)

	// Claim land.
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

	// Activate access pass law.
	lawID := proposeLaw(t, h, owner, landID, "ACCESS_PASS_CORE", map[string]interface{}{
		"ticket_item": "IRON_INGOT",
		"ticket_cost": 2,
	})
	voteYesAndActivate(t, h, owner, lawID)

	// Ensure the path from outside->core is unobstructed (tests should not depend on worldgen).
	for dx := 0; dx <= 17; dx++ {
		h.SetBlock(world.Vec3i{X: anchor.X + dx, Y: 0, Z: anchor.Z}, "AIR")
	}

	// Place visitor just outside core (dx=17), within claim.
	start := world.Vec3i{X: anchor.X + 17, Y: 0, Z: anchor.Z}
	h.SetAgentPosFor(visitor, start)
	h.ClearAgentEventsFor(visitor)

	h.StepFor(visitor, nil, []protocol.TaskReq{{
		ID:        "K_move",
		Type:      "MOVE_TO",
		Target:    anchorArr,
		Tolerance: 1.2,
	}}, nil)

	if got := invCount(h.LastObsFor(visitor).Inventory, "IRON_INGOT"); got != 3 {
		t.Fatalf("ticket charge: got %d want %d", got, 3)
	}
	if got := invCount(h.LastObsFor(owner).Inventory, "IRON_INGOT"); got != 2 {
		t.Fatalf("ticket credit: got %d want %d", got, 2)
	}
}

func TestLaw_AccessPassCore_BlocksIfInsufficientTicket(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:            "test",
		WorldType:     "OVERWORLD",
		TickRateHz:    5,
		DayTicks:      6000,
		ObsRadius:     7,
		Height:        1,
		Seed:          42,
		BoundaryR:     4000,
		LawNoticeTicks: 1,
		LawVoteTicks:   1,
		StarterItems: map[string]int{
			"BATTERY":      5,
			"CRYSTAL_SHARD": 5,
		},
	}, cats, "owner")
	owner := h.DefaultAgentID
	visitor := h.Join("visitor")

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
	if got := actionResultCode(h.LastObsFor(owner), "K_claim"); got != "" {
		t.Fatalf("claim expected ok, got code=%q", got)
	}
	landID := actionResultFieldString(h.LastObsFor(owner), "K_claim", "land_id")
	if landID == "" {
		t.Fatalf("expected land_id in claim ACTION_RESULT")
	}

	// Activate access pass law.
	lawID := proposeLaw(t, h, owner, landID, "ACCESS_PASS_CORE", map[string]interface{}{
		"ticket_item": "IRON_INGOT",
		"ticket_cost": 2,
	})
	voteYesAndActivate(t, h, owner, lawID)

	// Ensure the path from outside->core is unobstructed (tests should not depend on worldgen).
	for dx := 0; dx <= 17; dx++ {
		h.SetBlock(world.Vec3i{X: anchor.X + dx, Y: 0, Z: anchor.Z}, "AIR")
	}

	start := world.Vec3i{X: anchor.X + 17, Y: 0, Z: anchor.Z}
	h.SetAgentPosFor(visitor, start)
	h.AddInventoryFor(visitor, "IRON_INGOT", -9999)
	h.AddInventoryFor(visitor, "IRON_INGOT", 1)

	h.ClearAgentEventsFor(visitor)
	h.StepFor(visitor, nil, []protocol.TaskReq{{
		ID:        "K_move",
		Type:      "MOVE_TO",
		Target:    anchorArr,
		Tolerance: 1.2,
	}}, nil)

	obs := h.LastObsFor(visitor)
	if hasMoveTask(obs) {
		t.Fatalf("move task should fail/cancel when ticket missing")
	}
	if got := obs.Self.Pos; got != [3]int{start.X, 0, start.Z} {
		t.Fatalf("position should not change when ticket missing: got %+v want %+v", got, [3]int{start.X, 0, start.Z})
	}
}

func proposeLaw(t *testing.T, h *Harness, proposerID string, landID string, templateID string, params map[string]interface{}) string {
	t.Helper()
	h.ClearAgentEventsFor(proposerID)
	h.StepFor(proposerID, []protocol.InstantReq{{
		ID:         "I_prop",
		Type:       "PROPOSE_LAW",
		LandID:     landID,
		TemplateID: templateID,
		Params:     params,
	}}, nil, nil)
	lawID := actionResultFieldString(h.LastObsFor(proposerID), "I_prop", "law_id")
	if lawID == "" {
		t.Fatalf("expected law_id in propose ACTION_RESULT")
	}
	return lawID
}

func voteYesAndActivate(t *testing.T, h *Harness, voterID string, lawID string) {
	t.Helper()
	// Law transitions NOTICE->VOTING in tickLaws(), which runs after ACT application.
	// With the default governance timings, advance one tick so the law enters VOTING.
	h.StepNoop()

	h.ClearAgentEventsFor(voterID)
	h.StepFor(voterID, []protocol.InstantReq{{
		ID:     "I_vote",
		Type:   "VOTE",
		LawID:  lawID,
		Choice: "YES",
	}}, nil, nil)
	if got := actionResultCode(h.LastObsFor(voterID), "I_vote"); got != "" {
		t.Fatalf("vote expected ok, got code=%q", got)
	}
}

func hasMoveTask(obs protocol.ObsMsg) bool {
	for _, t := range obs.Tasks {
		if t.Kind == "MOVE_TO" {
			return true
		}
	}
	return false
}
