package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestClaimMaintenance_PaidResetsStageAndAdvancesDue(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:         "test",
		WorldType:  "OVERWORLD",
		TickRateHz: 5,
		DayTicks:   10,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
		StarterItems: map[string]int{
			"BATTERY":      5,
			"CRYSTAL_SHARD": 5,
		},
	}, cats, "owner")
	owner := h.DefaultAgentID

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")

	h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:     "K_claim",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 32,
	}}, nil)
	if got := actionResultCode(h.LastObsFor(owner), "K_claim"); got != "" {
		t.Fatalf("claim expected ok, got code=%q", got)
	}

	// Ensure unpaid at the first due.
	h.AddInventoryFor(owner, "IRON_INGOT", -9999)
	h.AddInventoryFor(owner, "COAL", -9999)

	firstDue := h.LastObsFor(owner).LocalRules.MaintenanceDueTick
	stepUntilTick(t, h, firstDue)
	if got := h.LastObsFor(owner).LocalRules.MaintenanceStage; got != 1 {
		t.Fatalf("stage after first miss: got %d want %d", got, 1)
	}

	// Pay before the second due.
	secondDue := h.LastObsFor(owner).LocalRules.MaintenanceDueTick
	h.AddInventoryFor(owner, "IRON_INGOT", 1)
	h.AddInventoryFor(owner, "COAL", 1)
	stepUntilTick(t, h, secondDue)

	obs := h.LastObsFor(owner)
	if got := obs.LocalRules.MaintenanceStage; got != 0 {
		t.Fatalf("stage after payment: got %d want %d", got, 0)
	}
	if got := obs.LocalRules.MaintenanceDueTick; got != secondDue+uint64(h.W.Config().DayTicks) {
		t.Fatalf("due tick after payment: got %d want %d", got, secondDue+uint64(h.W.Config().DayTicks))
	}
	if got := invCount(obs.Inventory, "IRON_INGOT"); got != 0 {
		t.Fatalf("iron deducted: got %d want %d", got, 0)
	}
	if got := invCount(obs.Inventory, "COAL"); got != 0 {
		t.Fatalf("coal deducted: got %d want %d", got, 0)
	}
}

func TestClaimMaintenance_UnpaidDowngradesToUnprotected(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:         "test",
		WorldType:  "OVERWORLD",
		TickRateHz: 5,
		DayTicks:   10,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
		StarterItems: map[string]int{
			"BATTERY":      5,
			"CRYSTAL_SHARD": 5,
			"CRAFTING_BENCH": 1,
		},
	}, cats, "owner")
	owner := h.DefaultAgentID
	visitor := h.Join("visitor")

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")

	h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:     "K_claim",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 32,
	}}, nil)
	if got := actionResultCode(h.LastObsFor(owner), "K_claim"); got != "" {
		t.Fatalf("claim expected ok, got code=%q", got)
	}

	// Ensure unpaid.
	h.AddInventoryFor(owner, "IRON_INGOT", -9999)
	h.AddInventoryFor(owner, "COAL", -9999)

	// Put visitor in the land.
	h.SetAgentPosFor(visitor, anchor)

	// Miss two dues -> stage 2.
	firstDue := h.LastObsFor(owner).LocalRules.MaintenanceDueTick
	stepUntilTick(t, h, firstDue)
	secondDue := h.LastObsFor(owner).LocalRules.MaintenanceDueTick
	stepUntilTick(t, h, secondDue)

	obsV := h.LastObsFor(visitor)
	if got := obsV.LocalRules.MaintenanceStage; got != 2 {
		t.Fatalf("stage after second miss: got %d want %d", got, 2)
	}
	if obsV.LocalRules.Permissions == nil || !obsV.LocalRules.Permissions["can_build"] || !obsV.LocalRules.Permissions["can_break"] || !obsV.LocalRules.Permissions["can_trade"] {
		t.Fatalf("expected unprotected perms for visitor, got %+v", obsV.LocalRules.Permissions)
	}

	// Visitor can place inside an unprotected claim.
	p := world.Vec3i{X: anchor.X + 3, Y: 0, Z: anchor.Z}
	h.SetBlock(p, "AIR")
	h.ClearAgentEventsFor(visitor)
	h.StepFor(visitor, nil, []protocol.TaskReq{{
		ID:       "K_place",
		Type:     "PLACE",
		ItemID:   "CRAFTING_BENCH",
		BlockPos: p.ToArray(),
	}}, nil)
	if hasTaskFail(h.LastObsFor(visitor), "") {
		t.Fatalf("expected PLACE to succeed in unprotected claim; events=%v", h.LastObsFor(visitor).Events)
	}
}
