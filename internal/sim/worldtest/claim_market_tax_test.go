package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

type tradeAuditSink struct {
	entries []world.AuditEntry
}

func (s *tradeAuditSink) WriteAudit(e world.AuditEntry) error {
	s.entries = append(s.entries, e)
	return nil
}

func lastAcceptedTradeAudit(entries []world.AuditEntry) *world.AuditEntry {
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.Action == "TRADE" && e.Reason == "ACCEPT_TRADE" {
			return &e
		}
	}
	return nil
}

func calcTaxF(count int, rate float64) int {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	tax := int(float64(count) * rate)
	if tax < 0 {
		return 0
	}
	if tax > count {
		return count
	}
	return tax
}

func TestTradeMarketTax_ToOwnerInventory(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:            "test",
		Seed:          1,
		LawNoticeTicks: 1,
		LawVoteTicks:   1,
	}, cats, "owner")
	owner := h.DefaultAgentID
	seller := h.Join("seller")
	buyer := h.Join("buyer")

	audit := &tradeAuditSink{}
	h.W.SetAuditLogger(audit)

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	clearArea(t, h, anchor, 2)

	// Claim land at anchor.
	h.AddInventoryFor(owner, "BATTERY", 1)
	h.AddInventoryFor(owner, "CRYSTAL_SHARD", 1)
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

	// Add both traders as members so can_trade is guaranteed regardless of visitor flags.
	h.ClearAgentEventsFor(owner)
	obs = h.StepFor(owner, []protocol.InstantReq{{
		ID:       "I_add_seller",
		Type:     "ADD_MEMBER",
		LandID:   landID,
		MemberID: seller,
	}, {
		ID:       "I_add_buyer",
		Type:     "ADD_MEMBER",
		LandID:   landID,
		MemberID: buyer,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_add_seller"); got != "" {
		t.Fatalf("ADD_MEMBER(seller) expected ok, got code=%q events=%v", got, obs.Events)
	}
	if got := actionResultCode(obs, "I_add_buyer"); got != "" {
		t.Fatalf("ADD_MEMBER(buyer) expected ok, got code=%q events=%v", got, obs.Events)
	}

	// Activate market tax (may be adjusted by active world events such as MARKET_WEEK).
	obs = h.StepFor(owner, []protocol.InstantReq{{
		ID:         "I_prop",
		Type:       "PROPOSE_LAW",
		LandID:     landID,
		TemplateID: "MARKET_TAX",
		Title:      "tax",
		Params:     map[string]interface{}{"market_tax": 0.10},
	}}, nil, nil)
	if got := actionResultCode(obs, "I_prop"); got != "" {
		t.Fatalf("PROPOSE_LAW expected ok, got code=%q events=%v", got, obs.Events)
	}
	lawID := actionResultFieldString(obs, "I_prop", "law_id")
	if lawID == "" {
		t.Fatalf("missing law_id; events=%v", obs.Events)
	}
	h.StepNoop() // NOTICE -> VOTING
	obs = h.StepFor(owner, []protocol.InstantReq{{
		ID:     "I_vote",
		Type:   "VOTE",
		LawID:  lawID,
		Choice: "YES",
	}}, nil, nil)
	if got := actionResultCode(obs, "I_vote"); got != "" {
		t.Fatalf("VOTE expected ok, got code=%q events=%v", got, obs.Events)
	}
	h.StepNoop() // finalize + apply

	// Put both inside the land.
	h.SetAgentPosFor(seller, anchor)
	h.SetAgentPosFor(buyer, anchor)
	h.StepNoop()

	// Reset inventories (only PLANK/IRON_INGOT matter here).
	h.AddInventoryFor(owner, "PLANK", -1_000_000)
	h.AddInventoryFor(owner, "IRON_INGOT", -1_000_000)
	h.AddInventoryFor(seller, "PLANK", -1_000_000)
	h.AddInventoryFor(seller, "IRON_INGOT", -1_000_000)
	h.AddInventoryFor(buyer, "PLANK", -1_000_000)
	h.AddInventoryFor(buyer, "IRON_INGOT", -1_000_000)

	const tradeCount = 20
	h.AddInventoryFor(seller, "PLANK", tradeCount)
	h.AddInventoryFor(buyer, "IRON_INGOT", tradeCount)

	// Offer and accept trade.
	obsOffer := h.StepFor(seller, []protocol.InstantReq{{
		ID:      "I_offer",
		Type:    "OFFER_TRADE",
		To:      buyer,
		Offer:   [][]interface{}{{"PLANK", tradeCount}},
		Request: [][]interface{}{{"IRON_INGOT", tradeCount}},
	}}, nil, nil)
	if got := actionResultCode(obsOffer, "I_offer"); got != "" {
		t.Fatalf("OFFER_TRADE expected ok, got code=%q events=%v", got, obsOffer.Events)
	}
	tradeID := actionResultFieldString(obsOffer, "I_offer", "trade_id")
	if tradeID == "" {
		t.Fatalf("missing trade_id; events=%v", obsOffer.Events)
	}

	_ = h.StepFor(buyer, []protocol.InstantReq{{
		ID:      "I_accept",
		Type:    "ACCEPT_TRADE",
		TradeID: tradeID,
	}}, nil, nil)

	// Read effective tax from audit.
	a := lastAcceptedTradeAudit(audit.entries)
	if a == nil || a.Details == nil {
		t.Fatalf("missing TRADE audit entry; entries=%d", len(audit.entries))
	}
	if got := a.Details["trade_id"]; got != tradeID {
		t.Fatalf("audit trade_id mismatch: got=%v want=%v", got, tradeID)
	}
	rate, _ := a.Details["tax_rate"].(float64)
	taxPerSide := calcTaxF(tradeCount, rate)
	if taxPerSide <= 0 {
		t.Fatalf("expected taxed trade to pay at least 1 item; taxRate=%v", rate)
	}
	recv := tradeCount - taxPerSide

	// Tax should go to land owner inventory (not an org).
	sellerIron := invCount(h.LastObsFor(seller).Inventory, "IRON_INGOT")
	buyerPlank := invCount(h.LastObsFor(buyer).Inventory, "PLANK")
	ownerPlank := invCount(h.LastObsFor(owner).Inventory, "PLANK")
	ownerIron := invCount(h.LastObsFor(owner).Inventory, "IRON_INGOT")

	if sellerIron != recv {
		t.Fatalf("seller iron: got %d want %d (rate=%v taxPerSide=%d)", sellerIron, recv, rate, taxPerSide)
	}
	if buyerPlank != recv {
		t.Fatalf("buyer plank: got %d want %d (rate=%v taxPerSide=%d)", buyerPlank, recv, rate, taxPerSide)
	}
	if ownerPlank != taxPerSide || ownerIron != taxPerSide {
		t.Fatalf("owner tax mismatch: plank=%d iron=%d want=%d (rate=%v)", ownerPlank, ownerIron, taxPerSide, rate)
	}
}

