package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestClaimType_HomesteadVisitorPermissions(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:        "OVERWORLD",
		WorldType: "OVERWORLD",
		Seed:      1,
		BoundaryR: 200,
	}, cats, "owner")
	owner := h.DefaultAgentID
	visitor := h.Join("visitor")

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")
	h.AddInventoryFor(owner, "BATTERY", 1)
	h.AddInventoryFor(owner, "CRYSTAL_SHARD", 1)

	h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:     "K_claim",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 16,
	}}, nil)
	if got := actionResultCode(h.LastObsFor(owner), "K_claim"); got != "" {
		t.Fatalf("claim expected ok, got code=%q", got)
	}

	h.SetAgentPosFor(visitor, anchor)
	h.StepNoop()
	perms := h.LastObsFor(visitor).LocalRules.Permissions
	if perms["can_build"] || perms["can_break"] {
		t.Fatalf("homestead visitor should not build/break: %+v", perms)
	}
}

func TestClaimType_CityCoreTradeAndDamage(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:        "CITY_HUB",
		WorldType: "CITY_HUB",
		Seed:      2,
		BoundaryR: 200,
	}, cats, "owner")
	owner := h.DefaultAgentID
	visitor := h.Join("visitor")

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")
	h.AddInventoryFor(owner, "BATTERY", 1)
	h.AddInventoryFor(owner, "CRYSTAL_SHARD", 1)

	h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:     "K_claim",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 16,
	}}, nil)
	if got := actionResultCode(h.LastObsFor(owner), "K_claim"); got != "" {
		t.Fatalf("claim expected ok, got code=%q", got)
	}

	h.SetAgentPosFor(visitor, anchor)
	h.StepNoop()
	perms := h.LastObsFor(visitor).LocalRules.Permissions
	if !perms["can_trade"] {
		t.Fatalf("city core visitor should trade by default: %+v", perms)
	}
	if perms["can_damage"] {
		t.Fatalf("city core visitor should not damage by default: %+v", perms)
	}
}

func TestClaimType_SnapshotRoundTrip_BehaviorPreserved(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	cfg := world.WorldConfig{
		ID:        "CITY_HUB",
		WorldType: "CITY_HUB",
		Seed:      3,
		BoundaryR: 200,
	}

	h1 := NewHarness(t, cfg, cats, "owner")
	owner := h1.DefaultAgentID

	anchorArr := h1.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h1.SetBlock(anchor, "AIR")
	h1.AddInventoryFor(owner, "BATTERY", 1)
	h1.AddInventoryFor(owner, "CRYSTAL_SHARD", 1)
	h1.StepFor(owner, nil, []protocol.TaskReq{{
		ID:     "K_claim",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 16,
	}}, nil)
	if got := actionResultCode(h1.LastObsFor(owner), "K_claim"); got != "" {
		t.Fatalf("claim expected ok, got code=%q", got)
	}

	_, snap := h1.Snapshot()

	w2, err := world.New(cfg, cats)
	if err != nil {
		t.Fatalf("world.New: %v", err)
	}
	if err := w2.ImportSnapshot(snap); err != nil {
		t.Fatalf("ImportSnapshot: %v", err)
	}
	h2 := NewHarnessWithWorld(t, w2, cats, "visitor")
	h2.SetAgentPos(anchor)
	h2.StepNoop()

	perms := h2.LastObs().LocalRules.Permissions
	if !perms["can_trade"] {
		t.Fatalf("expected can_trade preserved after snapshot import: %+v", perms)
	}
}

