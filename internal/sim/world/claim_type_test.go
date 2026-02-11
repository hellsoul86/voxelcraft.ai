package world

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/world/feature/governance"
)

func TestClaimType_HomesteadVisitorPermissions(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "OVERWORLD",
		WorldType:  "OVERWORLD",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       1,
		BoundaryR:  200,
	}, cats)
	if err != nil {
		t.Fatalf("new world: %v", err)
	}
	owner := &Agent{ID: "A1", Pos: Vec3i{X: 0, Y: 0, Z: 0}}
	visitor := &Agent{ID: "A2", Pos: owner.Pos}
	w.agents[owner.ID] = owner
	w.agents[visitor.ID] = visitor
	homeFlags := governance.DefaultClaimFlags(ClaimTypeHomestead)
	w.claims["LAND_HOME"] = &LandClaim{
		LandID:    "LAND_HOME",
		Owner:     owner.ID,
		ClaimType: ClaimTypeHomestead,
		Anchor:    owner.Pos,
		Radius:    16,
		Flags: ClaimFlags{
			AllowBuild:  homeFlags.AllowBuild,
			AllowBreak:  homeFlags.AllowBreak,
			AllowDamage: homeFlags.AllowDamage,
			AllowTrade:  homeFlags.AllowTrade,
		},
	}
	_, perms := w.permissionsFor(visitor.ID, owner.Pos)
	if perms["can_build"] || perms["can_break"] {
		t.Fatalf("homestead visitor should not build/break: %+v", perms)
	}
}

func TestClaimType_CityCoreTradeAndDamage(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "CITY_HUB",
		WorldType:  "CITY_HUB",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       2,
		BoundaryR:  200,
	}, cats)
	if err != nil {
		t.Fatalf("new world: %v", err)
	}
	owner := &Agent{ID: "A1", Pos: Vec3i{X: 0, Y: 0, Z: 0}}
	visitor := &Agent{ID: "A2", Pos: owner.Pos}
	w.agents[owner.ID] = owner
	w.agents[visitor.ID] = visitor
	cityFlags := governance.DefaultClaimFlags(ClaimTypeCityCore)
	w.claims["LAND_CITY"] = &LandClaim{
		LandID:    "LAND_CITY",
		Owner:     owner.ID,
		ClaimType: ClaimTypeCityCore,
		Anchor:    owner.Pos,
		Radius:    16,
		Flags: ClaimFlags{
			AllowBuild:  cityFlags.AllowBuild,
			AllowBreak:  cityFlags.AllowBreak,
			AllowDamage: cityFlags.AllowDamage,
			AllowTrade:  cityFlags.AllowTrade,
		},
	}
	_, perms := w.permissionsFor(visitor.ID, owner.Pos)
	if !perms["can_trade"] {
		t.Fatalf("city core visitor should be allowed to trade by default: %+v", perms)
	}
	if perms["can_damage"] {
		t.Fatalf("city core visitor should not damage by default: %+v", perms)
	}
}

func TestClaimType_SnapshotRoundTrip(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w1, err := New(WorldConfig{
		ID:         "OVERWORLD",
		WorldType:  "OVERWORLD",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       3,
		BoundaryR:  200,
	}, cats)
	if err != nil {
		t.Fatalf("new world1: %v", err)
	}
	cityFlags := governance.DefaultClaimFlags(ClaimTypeCityCore)
	w1.claims["LAND_X"] = &LandClaim{
		LandID:    "LAND_X",
		Owner:     "A1",
		ClaimType: ClaimTypeCityCore,
		Anchor:    Vec3i{X: 1, Y: 0, Z: 1},
		Radius:    8,
		Flags: ClaimFlags{
			AllowBuild:  cityFlags.AllowBuild,
			AllowBreak:  cityFlags.AllowBreak,
			AllowDamage: cityFlags.AllowDamage,
			AllowTrade:  cityFlags.AllowTrade,
		},
	}
	snap := w1.ExportSnapshot(0)

	w2, err := New(WorldConfig{
		ID:         "OVERWORLD",
		WorldType:  "OVERWORLD",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       3,
		BoundaryR:  200,
	}, cats)
	if err != nil {
		t.Fatalf("new world2: %v", err)
	}
	if err := w2.ImportSnapshot(snap); err != nil {
		t.Fatalf("import snapshot: %v", err)
	}
	got := w2.claims["LAND_X"]
	if got == nil {
		t.Fatalf("claim missing after import")
	}
	if got.ClaimType != ClaimTypeCityCore {
		t.Fatalf("claim_type mismatch after roundtrip: got %q", got.ClaimType)
	}
}
