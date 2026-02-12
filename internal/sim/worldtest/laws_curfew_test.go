package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestLaw_CurfewNoBuild_DeniesPlaceDuringWindow(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:             "test",
		WorldType:      "OVERWORLD",
		TickRateHz:     5,
		DayTicks:       100,
		ObsRadius:      7,
		Height:         1,
		Seed:           1,
		BoundaryR:      4000,
		LawNoticeTicks: 1,
		LawVoteTicks:   1,
		StarterItems: map[string]int{
			"BATTERY":      5,
			"CRYSTAL_SHARD": 5,
			"SIGN":         5,
		},
	}, cats, "owner")
	owner := h.DefaultAgentID

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")

	// Claim land at anchor.
	h.ClearAgentEventsFor(owner)
	obs := h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:     "K_claim",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 32,
	}}, nil)
	if got := actionResultCode(obs, "K_claim"); got != "" {
		t.Fatalf("CLAIM_LAND expected ok, got code=%q events=%v", got, obs.Events)
	}
	landID := actionResultFieldString(obs, "K_claim", "land_id")
	if landID == "" {
		t.Fatalf("missing land_id; events=%v", obs.Events)
	}

	// Propose + vote YES to activate curfew.
	h.ClearAgentEventsFor(owner)
	obs = h.StepFor(owner, []protocol.InstantReq{{
		ID:         "I_prop",
		Type:       "PROPOSE_LAW",
		LandID:     landID,
		TemplateID: "CURFEW_NO_BUILD",
		Title:      "curfew",
		Params: map[string]interface{}{
			"start_time": 0.0,
			"end_time":   0.1,
		},
	}}, nil, nil)
	if got := actionResultCode(obs, "I_prop"); got != "" {
		t.Fatalf("PROPOSE_LAW expected ok, got code=%q events=%v", got, obs.Events)
	}
	lawID := actionResultFieldString(obs, "I_prop", "law_id")
	if lawID == "" {
		t.Fatalf("missing law_id; events=%v", obs.Events)
	}

	h.StepNoop() // NOTICE -> VOTING (LawNoticeTicks=1)
	obs = h.StepFor(owner, []protocol.InstantReq{{
		ID:     "I_vote",
		Type:   "VOTE",
		LawID:  lawID,
		Choice: "YES",
	}}, nil, nil)
	if got := actionResultCode(obs, "I_vote"); got != "" {
		t.Fatalf("VOTE expected ok, got code=%q events=%v", got, obs.Events)
	}
	h.StepNoop() // finalize + apply (LawVoteTicks=1)

	// Find a tick where time_of_day is inside curfew window.
	for i := 0; i < 300; i++ {
		tod := h.LastObsFor(owner).World.TimeOfDay
		if tod >= 0.0 && tod <= 0.1 {
			break
		}
		h.StepNoop()
	}
	if tod := h.LastObsFor(owner).World.TimeOfDay; !(tod >= 0.0 && tod <= 0.1) {
		t.Fatalf("expected time_of_day in [0.0,0.1], got %v", tod)
	}

	// Attempt to place inside land: should be denied.
	pos := world.Vec3i{X: anchor.X + 1, Y: 0, Z: anchor.Z}
	h.SetBlock(pos, "AIR")
	before := invCount(h.LastObsFor(owner).Inventory, "SIGN")

	h.ClearAgentEventsFor(owner)
	obs = h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:       "K_place",
		Type:     "PLACE",
		ItemID:   "SIGN",
		BlockPos: pos.ToArray(),
	}}, nil)
	if !hasTaskFail(obs, "E_NO_PERMISSION") {
		t.Fatalf("expected TASK_FAIL E_NO_PERMISSION during curfew, got events=%v", obs.Events)
	}
	if got := invCount(h.LastObsFor(owner).Inventory, "SIGN"); got != before {
		t.Fatalf("expected inventory unchanged on denied place, got %d want %d", got, before)
	}
	b, err := h.W.DebugGetBlock(pos)
	if err != nil {
		t.Fatalf("DebugGetBlock: %v", err)
	}
	if gotName := h.Cats.Blocks.Palette[b]; gotName != "AIR" {
		t.Fatalf("block should remain AIR when denied: got %q", gotName)
	}

	// Outside the curfew window, placement should succeed.
	for i := 0; i < 300; i++ {
		tod := h.LastObsFor(owner).World.TimeOfDay
		if tod > 0.1 {
			break
		}
		h.StepNoop()
	}
	if tod := h.LastObsFor(owner).World.TimeOfDay; !(tod > 0.1) {
		t.Fatalf("expected time_of_day > 0.1 outside curfew, got %v", tod)
	}

	h.ClearAgentEventsFor(owner)
	obs = h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:       "K_place_ok",
		Type:     "PLACE",
		ItemID:   "SIGN",
		BlockPos: pos.ToArray(),
	}}, nil)
	if hasTaskFail(obs, "") {
		t.Fatalf("expected place outside curfew to succeed, got events=%v", obs.Events)
	}
	if got := invCount(h.LastObsFor(owner).Inventory, "SIGN"); got != before-1 {
		t.Fatalf("expected inventory decremented by 1 on success, got %d want %d", got, before-1)
	}
	b, err = h.W.DebugGetBlock(pos)
	if err != nil {
		t.Fatalf("DebugGetBlock: %v", err)
	}
	if gotName := h.Cats.Blocks.Palette[b]; gotName != "SIGN" {
		t.Fatalf("block should be SIGN after successful place: got %q", gotName)
	}
}

