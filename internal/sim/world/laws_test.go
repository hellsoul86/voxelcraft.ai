package world

import (
	"math"
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/world/feature/governance"
)

func TestLawLifecycleAndActivation(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   100,
		ObsRadius:  7,
		Height:     1,
		Seed:       1,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	out := make(chan []byte, 1)
	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "owner", DeltaVoxels: false, Out: out, Resp: resp})
	jr := <-resp
	owner := w.agents[jr.Welcome.AgentID]

	land := &LandClaim{
		LandID: "LAND_LAW",
		Owner:  owner.ID,
		Anchor: owner.Pos,
		Radius: 32,
		Flags:  ClaimFlags{AllowBuild: true, AllowBreak: true, AllowDamage: false, AllowTrade: true},
	}
	w.claims[land.LandID] = land

	// Propose market tax.
	w.applyInstant(owner, protocol.InstantReq{
		ID:         "I_prop_tax",
		Type:       "PROPOSE_LAW",
		LandID:     land.LandID,
		TemplateID: "MARKET_TAX",
		Params:     map[string]interface{}{"market_tax": 0.10},
		Title:      "Tax 10%",
	}, 0)

	var taxLawID string
	for id, l := range w.laws {
		if l.TemplateID == "MARKET_TAX" {
			taxLawID = id
			break
		}
	}
	if taxLawID == "" {
		t.Fatalf("expected market tax law id")
	}

	// Move through NOTICE -> VOTING.
	w.tickLaws(2999)
	if w.laws[taxLawID].Status != LawNotice {
		t.Fatalf("expected NOTICE, got %s", w.laws[taxLawID].Status)
	}
	w.tickLaws(3000)
	if w.laws[taxLawID].Status != LawVoting {
		t.Fatalf("expected VOTING, got %s", w.laws[taxLawID].Status)
	}

	// Vote YES and finalize.
	w.applyInstant(owner, protocol.InstantReq{
		ID:     "I_vote_yes",
		Type:   "VOTE",
		LawID:  taxLawID,
		Choice: "YES",
	}, 3001)
	w.tickLaws(6000)

	if w.laws[taxLawID].Status != LawActive {
		t.Fatalf("expected ACTIVE, got %s", w.laws[taxLawID].Status)
	}
	if math.Abs(land.MarketTax-0.10) > 1e-9 {
		t.Fatalf("market tax: got %f want %f", land.MarketTax, 0.10)
	}

	// Propose curfew and activate.
	w.applyInstant(owner, protocol.InstantReq{
		ID:         "I_prop_curfew",
		Type:       "PROPOSE_LAW",
		LandID:     land.LandID,
		TemplateID: "CURFEW_NO_BUILD",
		Params:     map[string]interface{}{"start_time": 0.0, "end_time": 0.1},
		Title:      "Curfew 0-0.1",
	}, 7000)
	var curfewLawID string
	for id, l := range w.laws {
		if l.TemplateID == "CURFEW_NO_BUILD" {
			curfewLawID = id
			break
		}
	}
	if curfewLawID == "" {
		t.Fatalf("expected curfew law id")
	}

	w.tickLaws(10000) // 7000+3000 => voting
	if w.laws[curfewLawID].Status != LawVoting {
		t.Fatalf("expected VOTING, got %s", w.laws[curfewLawID].Status)
	}
	w.applyInstant(owner, protocol.InstantReq{
		ID:     "I_vote_yes_2",
		Type:   "VOTE",
		LawID:  curfewLawID,
		Choice: "YES",
	}, 10001)
	w.tickLaws(13000) // 7000+6000 => finalize

	if !land.CurfewEnabled {
		t.Fatalf("expected curfew enabled")
	}
	if !governance.InWindow(w.timeOfDay(5), land.CurfewStart, land.CurfewEnd) {
		t.Fatalf("expected tick 5 in curfew window")
	}
	if w.canBuildAt(owner.ID, owner.Pos, 5) {
		t.Fatalf("expected canBuildAt=false during curfew after activation")
	}
}
