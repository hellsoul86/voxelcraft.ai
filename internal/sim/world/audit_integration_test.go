package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

type memAudit struct {
	entries []AuditEntry
}

func (m *memAudit) WriteAudit(e AuditEntry) error {
	m.entries = append(m.entries, e)
	return nil
}

func TestAudit_TradeAccept_WritesTradeEntry(t *testing.T) {
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

	aud := &memAudit{}
	w.SetAuditLogger(aud)

	join := func(name string) *Agent {
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: name, DeltaVoxels: false, Out: nil, Resp: resp})
		jr := <-resp
		return w.agents[jr.Welcome.AgentID]
	}

	owner := join("owner")
	seller := join("seller")
	buyer := join("buyer")
	if owner == nil || seller == nil || buyer == nil {
		t.Fatalf("missing agents")
	}

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

	found := false
	for _, e := range aud.entries {
		if e.Action != "TRADE" {
			continue
		}
		found = true
		if e.Actor != buyer.ID {
			t.Fatalf("audit actor: got %q want %q", e.Actor, buyer.ID)
		}
		if e.Reason != "ACCEPT_TRADE" {
			t.Fatalf("audit reason: got %q want %q", e.Reason, "ACCEPT_TRADE")
		}
		if e.Details == nil || e.Details["trade_id"] != tradeID {
			t.Fatalf("missing trade_id in details: %+v", e.Details)
		}
		if rate, ok := e.Details["tax_rate"].(float64); !ok || rate != 0.10 {
			t.Fatalf("tax_rate: got %T=%v want %v", e.Details["tax_rate"], e.Details["tax_rate"], 0.10)
		}
		break
	}
	if !found {
		t.Fatalf("expected TRADE audit entry; got %d entries", len(aud.entries))
	}
}
