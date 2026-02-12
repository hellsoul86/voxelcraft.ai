package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestClaimUpgrade_OwnerHappyPath(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{ID: "test", WorldType: "OVERWORLD", Seed: 1}, cats, "owner")
	owner := h.DefaultAgentID

	// Exactly enough for claim (1+1) + upgrade 32->64 (1+2).
	h.AddInventoryFor(owner, "BATTERY", 2)
	h.AddInventoryFor(owner, "CRYSTAL_SHARD", 3)

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")

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
		t.Fatalf("missing land_id from claim ACTION_RESULT; events=%v", obs.Events)
	}

	h.ClearAgentEventsFor(owner)
	obs = h.StepFor(owner, []protocol.InstantReq{{
		ID:     "I_up",
		Type:   "UPGRADE_CLAIM",
		LandID: landID,
		Radius: 64,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_up"); got != "" {
		t.Fatalf("upgrade expected ok, got code=%q events=%v", got, obs.Events)
	}
	if got := actionResultFieldInt(obs, "I_up", "radius"); got != 64 {
		t.Fatalf("upgrade radius: got %d want %d events=%v", got, 64, obs.Events)
	}
	if got := invCount(obs.Inventory, "BATTERY"); got != 0 {
		t.Fatalf("battery after upgrade: got %d want 0", got)
	}
	if got := invCount(obs.Inventory, "CRYSTAL_SHARD"); got != 0 {
		t.Fatalf("crystal after upgrade: got %d want 0", got)
	}
}

func TestClaimUpgrade_RequiresAdminAndNoOverlap(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{ID: "test", WorldType: "OVERWORLD", Seed: 2}, cats, "a1")
	a1 := h.DefaultAgentID
	a2 := h.Join("a2")

	anchor1 := world.Vec3i{X: 0, Y: 0, Z: 0}
	anchor2 := world.Vec3i{X: 80, Y: 0, Z: 0} // does not overlap at radius=32, overlaps if anchor1 upgrades to 64
	h.SetBlock(anchor1, "AIR")
	h.SetBlock(anchor2, "AIR")
	h.SetAgentPosFor(a1, anchor1)
	h.SetAgentPosFor(a2, anchor2)

	h.AddInventoryFor(a1, "BATTERY", 10)
	h.AddInventoryFor(a1, "CRYSTAL_SHARD", 10)
	h.AddInventoryFor(a2, "BATTERY", 10)
	h.AddInventoryFor(a2, "CRYSTAL_SHARD", 10)

	h.StepFor(a1, nil, []protocol.TaskReq{{
		ID:     "K_claim1",
		Type:   "CLAIM_LAND",
		Anchor: anchor1.ToArray(),
		Radius: 32,
	}}, nil)
	obs1 := h.LastObsFor(a1)
	if got := actionResultCode(obs1, "K_claim1"); got != "" {
		t.Fatalf("claim1 expected ok, got code=%q events=%v", got, obs1.Events)
	}
	land1 := actionResultFieldString(obs1, "K_claim1", "land_id")
	if land1 == "" {
		t.Fatalf("missing land_id for claim1; events=%v", obs1.Events)
	}

	h.StepFor(a2, nil, []protocol.TaskReq{{
		ID:     "K_claim2",
		Type:   "CLAIM_LAND",
		Anchor: anchor2.ToArray(),
		Radius: 32,
	}}, nil)
	if got := actionResultCode(h.LastObsFor(a2), "K_claim2"); got != "" {
		t.Fatalf("claim2 expected ok, got code=%q", got)
	}

	// Non-admin cannot upgrade a1's claim.
	h.ClearAgentEventsFor(a2)
	obs2 := h.StepFor(a2, []protocol.InstantReq{{
		ID:     "I_up_bad",
		Type:   "UPGRADE_CLAIM",
		LandID: land1,
		Radius: 64,
	}}, nil, nil)
	if got := actionResultCode(obs2, "I_up_bad"); got != "E_NO_PERMISSION" {
		t.Fatalf("expected E_NO_PERMISSION, got code=%q events=%v", got, obs2.Events)
	}

	// Admin upgrade should fail due to overlap with land2.
	h.ClearAgentEventsFor(a1)
	obs1 = h.StepFor(a1, []protocol.InstantReq{{
		ID:     "I_up_overlap",
		Type:   "UPGRADE_CLAIM",
		LandID: land1,
		Radius: 64,
	}}, nil, nil)
	if got := actionResultCode(obs1, "I_up_overlap"); got != "E_CONFLICT" {
		t.Fatalf("expected E_CONFLICT, got code=%q events=%v", got, obs1.Events)
	}
}

func TestClaimUpgrade_BlockedByMaintenanceStageAndMaterials(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:         "test",
		WorldType:  "OVERWORLD",
		Seed:       3,
		DayTicks:   10,
		BoundaryR:  200,
		TickRateHz: 5,
	}, cats, "owner")
	owner := h.DefaultAgentID

	// Just enough for claim.
	h.AddInventoryFor(owner, "BATTERY", 1)
	h.AddInventoryFor(owner, "CRYSTAL_SHARD", 1)

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")

	h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:     "K_claim",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 32,
	}}, nil)
	obs := h.LastObsFor(owner)
	if got := actionResultCode(obs, "K_claim"); got != "" {
		t.Fatalf("claim expected ok, got code=%q", got)
	}
	landID := actionResultFieldString(obs, "K_claim", "land_id")
	if landID == "" {
		t.Fatalf("missing land_id from claim ACTION_RESULT; events=%v", obs.Events)
	}

	// Missing materials for upgrade.
	h.ClearAgentEventsFor(owner)
	obs = h.StepFor(owner, []protocol.InstantReq{{
		ID:     "I_up_missing",
		Type:   "UPGRADE_CLAIM",
		LandID: landID,
		Radius: 64,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_up_missing"); got != "E_NO_RESOURCE" {
		t.Fatalf("expected E_NO_RESOURCE, got code=%q events=%v", got, obs.Events)
	}

	// Miss first maintenance due -> stage 1.
	targetDue := h.LastObsFor(owner).LocalRules.MaintenanceDueTick
	stepUntilTick(t, h, targetDue)
	if got := h.LastObsFor(owner).LocalRules.MaintenanceStage; got != 1 {
		t.Fatalf("expected maintenance stage 1, got %d", got)
	}

	// Put upgrade materials in inventory, but stage 1 disallows expansion.
	h.AddInventoryFor(owner, "BATTERY", 1)
	h.AddInventoryFor(owner, "CRYSTAL_SHARD", 2)
	h.ClearAgentEventsFor(owner)
	obs = h.StepFor(owner, []protocol.InstantReq{{
		ID:     "I_up_stage",
		Type:   "UPGRADE_CLAIM",
		LandID: landID,
		Radius: 64,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_up_stage"); got != "E_NO_PERMISSION" {
		t.Fatalf("expected E_NO_PERMISSION, got code=%q events=%v", got, obs.Events)
	}
}

func actionResultFieldInt(obs protocol.ObsMsg, ref string, key string) int {
	for _, e := range obs.Events {
		if typ, _ := e["type"].(string); typ != "ACTION_RESULT" {
			continue
		}
		if got, _ := e["ref"].(string); got != ref {
			continue
		}
		switch v := e[key].(type) {
		case float64:
			return int(v)
		case int:
			return v
		}
	}
	return 0
}
