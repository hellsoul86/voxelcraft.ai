package world

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
)

func TestClaimMaintenance_PaidResetsStageAndAdvancesDue(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   10,
		ObsRadius:  7,
		Height:     64,
		Seed:       42,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "owner", DeltaVoxels: false, Out: nil, Resp: resp})
	r := <-resp
	owner := w.agents[r.Welcome.AgentID]
	if owner == nil {
		t.Fatalf("missing owner")
	}

	landID := w.newLandID(owner.ID)
	w.claims[landID] = &LandClaim{
		LandID:             landID,
		Owner:              owner.ID,
		Anchor:             owner.Pos,
		Radius:             32,
		Flags:              ClaimFlags{},
		Members:            map[string]bool{},
		MaintenanceDueTick: 1,
		MaintenanceStage:   1,
	}

	owner.Inventory["IRON_INGOT"] = 1
	owner.Inventory["COAL"] = 1

	// Tick 0: not due.
	w.step(nil, nil, nil)
	// Tick 1: due -> paid.
	w.step(nil, nil, nil)

	c := w.claims[landID]
	if c == nil {
		t.Fatalf("missing claim")
	}
	if got := c.MaintenanceStage; got != 0 {
		t.Fatalf("stage after payment: got %d want %d", got, 0)
	}
	if got := c.MaintenanceDueTick; got != 11 {
		t.Fatalf("due tick after payment: got %d want %d", got, 11)
	}
	if got := owner.Inventory["IRON_INGOT"]; got != 0 {
		t.Fatalf("iron deducted: got %d want %d", got, 0)
	}
	if got := owner.Inventory["COAL"]; got != 0 {
		t.Fatalf("coal deducted: got %d want %d", got, 0)
	}
}

func TestClaimMaintenance_UnpaidDowngradesToUnprotected(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   10,
		ObsRadius:  7,
		Height:     64,
		Seed:       42,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	join := func(name string) *Agent {
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: name, DeltaVoxels: false, Out: nil, Resp: resp})
		r := <-resp
		return w.agents[r.Welcome.AgentID]
	}
	owner := join("owner")
	visitor := join("visitor")
	if owner == nil || visitor == nil {
		t.Fatalf("missing agents")
	}

	landID := w.newLandID(owner.ID)
	w.claims[landID] = &LandClaim{
		LandID:             landID,
		Owner:              owner.ID,
		Anchor:             owner.Pos,
		Radius:             32,
		Flags:              ClaimFlags{AllowBuild: false, AllowBreak: false, AllowDamage: false, AllowTrade: false},
		Members:            map[string]bool{},
		MaintenanceDueTick: 1,
		MaintenanceStage:   0,
	}

	// Ensure unpaid.
	owner.Inventory["IRON_INGOT"] = 0
	owner.Inventory["COAL"] = 0

	// Tick 0 -> 1 (due) unpaid => stage 1.
	w.step(nil, nil, nil)
	w.step(nil, nil, nil)
	if got := w.claims[landID].MaintenanceStage; got != 1 {
		t.Fatalf("stage after first miss: got %d want %d", got, 1)
	}

	// Advance to next due (tick 11).
	for w.CurrentTick() < 11 {
		w.step(nil, nil, nil)
	}
	// Tick 11 unpaid => stage 2 (unprotected).
	w.step(nil, nil, nil)
	if got := w.claims[landID].MaintenanceStage; got != 2 {
		t.Fatalf("stage after second miss: got %d want %d", got, 2)
	}

	_, perms := w.permissionsFor(visitor.ID, owner.Pos)
	if !perms["can_build"] || !perms["can_break"] || !perms["can_trade"] {
		t.Fatalf("expected unprotected perms for visitor: %+v", perms)
	}
}
