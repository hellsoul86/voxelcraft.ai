package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestCurfewBlocksBuildAndBreak(t *testing.T) {
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
	if owner == nil {
		t.Fatalf("expected owner agent")
	}

	land := &LandClaim{
		LandID:        "LAND_TEST",
		Owner:         owner.ID,
		Anchor:        owner.Pos,
		Radius:        32,
		Flags:         ClaimFlags{AllowBuild: true, AllowBreak: true, AllowDamage: false, AllowTrade: true},
		CurfewEnabled: true,
		CurfewStart:   0.0,
		CurfewEnd:     0.1,
	}
	w.claims[land.LandID] = land

	// nowTick=5 => time_of_day=0.05, inside curfew window.
	if w.canBuildAt(owner.ID, owner.Pos, 5) {
		t.Fatalf("expected canBuildAt=false during curfew")
	}
	if w.canBreakAt(owner.ID, owner.Pos, 5) {
		t.Fatalf("expected canBreakAt=false during curfew")
	}

	// nowTick=20 => time_of_day=0.2, outside curfew window.
	if !w.canBuildAt(owner.ID, owner.Pos, 20) {
		t.Fatalf("expected canBuildAt=true outside curfew")
	}
	if !w.canBreakAt(owner.ID, owner.Pos, 20) {
		t.Fatalf("expected canBreakAt=true outside curfew")
	}
}

func TestTradeMarketTax(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       1,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	join := func(name string) *Agent {
		out := make(chan []byte, 1)
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: name, DeltaVoxels: false, Out: out, Resp: resp})
		jr := <-resp
		return w.agents[jr.Welcome.AgentID]
	}

	owner := join("owner")
	seller := join("seller")
	buyer := join("buyer")

	// Reset inventories for predictable assertions.
	owner.Inventory = map[string]int{}
	seller.Inventory = map[string]int{}
	buyer.Inventory = map[string]int{}

	// Put everyone inside the same claim.
	seller.Pos = owner.Pos
	buyer.Pos = owner.Pos

	land := &LandClaim{
		LandID:    "LAND_TAX",
		Owner:     owner.ID,
		Anchor:    owner.Pos,
		Radius:    32,
		Flags:     ClaimFlags{AllowBuild: true, AllowBreak: true, AllowDamage: false, AllowTrade: true},
		MarketTax: 0.10,
	}
	w.claims[land.LandID] = land

	seller.Inventory["PLANK"] = 10
	buyer.Inventory["IRON_INGOT"] = 10

	// Seller offers 10 planks for 10 iron ingots.
	w.applyInstant(seller, protocol.InstantReq{
		ID:      "I_offer",
		Type:    "OFFER_TRADE",
		To:      buyer.ID,
		Offer:   [][]interface{}{{"PLANK", 10}},
		Request: [][]interface{}{{"IRON_INGOT", 10}},
	}, 0)

	var tradeID string
	for id := range w.trades {
		tradeID = id
		break
	}
	if tradeID == "" {
		t.Fatalf("expected trade id")
	}

	w.applyInstant(buyer, protocol.InstantReq{
		ID:      "I_accept",
		Type:    "ACCEPT_TRADE",
		TradeID: tradeID,
	}, 1)

	// 10% tax means each side receives 9 and 1 goes to the claim owner.
	if got := seller.Inventory["PLANK"]; got != 0 {
		t.Fatalf("seller planks: got %d want 0", got)
	}
	if got := seller.Inventory["IRON_INGOT"]; got != 9 {
		t.Fatalf("seller iron: got %d want 9", got)
	}
	if got := buyer.Inventory["IRON_INGOT"]; got != 0 {
		t.Fatalf("buyer iron: got %d want 0", got)
	}
	if got := buyer.Inventory["PLANK"]; got != 9 {
		t.Fatalf("buyer planks: got %d want 9", got)
	}
	if got := owner.Inventory["PLANK"]; got != 1 {
		t.Fatalf("owner planks (tax): got %d want 1", got)
	}
	if got := owner.Inventory["IRON_INGOT"]; got != 1 {
		t.Fatalf("owner iron (tax): got %d want 1", got)
	}
}
