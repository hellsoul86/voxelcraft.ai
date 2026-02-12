package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestOrgs_CreateJoinDeedAndTax(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:            "test",
		WorldType:     "OVERWORLD",
		Seed:          1,
		LawNoticeTicks: 1,
		LawVoteTicks:   1,
	}, cats, "leader")
	audit := &auditSink{}
	h.W.SetAuditLogger(audit)
	leader := h.DefaultAgentID
	member := h.Join("member")

	// Create org.
	obs := h.StepFor(leader, []protocol.InstantReq{{
		ID:      "I_create",
		Type:    "CREATE_ORG",
		OrgKind: "GUILD",
		OrgName: "RiverGuild",
	}}, nil, nil)
	if got := actionResultCode(obs, "I_create"); got != "" {
		t.Fatalf("CREATE_ORG expected ok, got code=%q events=%v", got, obs.Events)
	}
	orgID := actionResultFieldString(obs, "I_create", "org_id")
	if orgID == "" {
		t.Fatalf("missing org_id; events=%v", obs.Events)
	}

	// Join org.
	h.ClearAgentEventsFor(member)
	obsM := h.StepFor(member, []protocol.InstantReq{{
		ID:    "I_join",
		Type:  "JOIN_ORG",
		OrgID: orgID,
	}}, nil, nil)
	if got := actionResultCode(obsM, "I_join"); got != "" {
		t.Fatalf("JOIN_ORG expected ok, got code=%q events=%v", got, obsM.Events)
	}

	// Claim land and deed it to org.
	h.AddInventoryFor(leader, "BATTERY", 1)
	h.AddInventoryFor(leader, "CRYSTAL_SHARD", 1)
	anchorArr := h.LastObsFor(leader).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")
	obs = h.StepFor(leader, nil, []protocol.TaskReq{{
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

	h.ClearAgentEventsFor(leader)
	obs = h.StepFor(leader, []protocol.InstantReq{{
		ID:       "I_deed",
		Type:     "DEED_LAND",
		LandID:   landID,
		NewOwner: orgID,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_deed"); got != "" {
		t.Fatalf("DEED_LAND expected ok, got code=%q events=%v", got, obs.Events)
	}

	// Activate market tax.
	h.ClearAgentEventsFor(leader)
	obs = h.StepFor(leader, []protocol.InstantReq{{
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
	h.ClearAgentEventsFor(leader)
	obs = h.StepFor(leader, []protocol.InstantReq{{
		ID:     "I_vote",
		Type:   "VOTE",
		LawID:  lawID,
		Choice: "YES",
	}}, nil, nil)
	if got := actionResultCode(obs, "I_vote"); got != "" {
		t.Fatalf("VOTE expected ok, got code=%q events=%v", got, obs.Events)
	}
	h.StepNoop() // allow tickLaws to finalize and apply template
	if got := h.LastObsFor(leader).LocalRules.Tax["market"]; got < 0.099 || got > 0.101 {
		t.Fatalf("expected market tax ~0.10 after activation, got %v", got)
	}

	// Put both inside the land (trade permission depends on land membership).
	h.SetAgentPosFor(member, anchor)
	h.SetAgentPosFor(leader, anchor)
	h.StepNoop()
	if got := h.LastObsFor(leader).LocalRules.LandID; got != landID {
		t.Fatalf("leader expected inside land %q, got land_id=%q", landID, got)
	}
	if got := h.LastObsFor(member).LocalRules.LandID; got != landID {
		t.Fatalf("member expected inside land %q, got land_id=%q", landID, got)
	}
	if perms := h.LastObsFor(leader).LocalRules.Permissions; perms == nil || !perms["can_trade"] {
		t.Fatalf("leader expected can_trade=true, got perms=%v", perms)
	}
	if perms := h.LastObsFor(member).LocalRules.Permissions; perms == nil || !perms["can_trade"] {
		t.Fatalf("member expected can_trade=true, got perms=%v", perms)
	}
	if got := h.LastObsFor(leader).LocalRules.Tax["market"]; got < 0.099 || got > 0.101 {
		t.Fatalf("leader expected market tax ~0.10 inside land, got %v", got)
	}

	// Reset inventories for predictable assertions (only PLANK/IRON_INGOT matter here).
	h.AddInventoryFor(leader, "PLANK", -9999)
	h.AddInventoryFor(leader, "IRON_INGOT", -9999)
	h.AddInventoryFor(member, "PLANK", -9999)
	h.AddInventoryFor(member, "IRON_INGOT", -9999)
	h.AddInventoryFor(leader, "PLANK", 20)
	h.AddInventoryFor(member, "IRON_INGOT", 20)

	// Offer and accept trade.
	h.ClearAgentEventsFor(leader)
	obs = h.StepFor(leader, []protocol.InstantReq{{
		ID:      "I_offer",
		Type:    "OFFER_TRADE",
		To:      member,
		Offer:   [][]interface{}{{"PLANK", 20}},
		Request: [][]interface{}{{"IRON_INGOT", 20}},
	}}, nil, nil)
	if got := actionResultCode(obs, "I_offer"); got != "" {
		t.Fatalf("OFFER_TRADE expected ok, got code=%q events=%v", got, obs.Events)
	}
	tradeID := actionResultFieldString(obs, "I_offer", "trade_id")
	if tradeID == "" {
		t.Fatalf("missing trade_id; events=%v", obs.Events)
	}

	h.ClearAgentEventsFor(member)
	obsM = h.StepFor(member, []protocol.InstantReq{{
		ID:      "I_accept",
		Type:    "ACCEPT_TRADE",
		TradeID: tradeID,
	}}, nil, nil)
	if got := actionResultCode(obsM, "I_accept"); got != "" {
		t.Fatalf("ACCEPT_TRADE expected ok, got code=%q events=%v", got, obsM.Events)
	}

	// The effective tax rate may be adjusted by active world events (e.g. MARKET_WEEK halves the rate).
	taxRate := audit.lastTradeTaxRate()
	tradeCount := 20
	taxPerSide := calcTax(tradeCount, taxRate)
	recv := tradeCount - taxPerSide
	if taxPerSide <= 0 {
		t.Fatalf("expected taxed trade to pay at least 1 item; taxRate=%v", taxRate)
	}

	leaderIron := invCount(h.LastObsFor(leader).Inventory, "IRON_INGOT")
	memberPlank := invCount(h.LastObsFor(member).Inventory, "PLANK")
	if leaderIron != recv || memberPlank != recv {
		t.Fatalf("taxed trade mismatch: leaderIron=%d memberPlank=%d recv=%d taxPerSide=%d landLeader=%q landMember=%q taxLeader=%v auditTaxRate=%v",
			leaderIron,
			memberPlank,
			recv,
			taxPerSide,
			h.LastObsFor(leader).LocalRules.LandID,
			h.LastObsFor(member).LocalRules.LandID,
			h.LastObsFor(leader).LocalRules.Tax,
			taxRate,
		)
	}

	// Org treasury withdraw: only admins.
	h.ClearAgentEventsFor(member)
	obsM = h.StepFor(member, []protocol.InstantReq{{
		ID:     "I_withdraw_bad",
		Type:   "ORG_WITHDRAW",
		OrgID:  orgID,
		ItemID: "PLANK",
		Count:  taxPerSide,
	}}, nil, nil)
	if got := actionResultCode(obsM, "I_withdraw_bad"); got != "E_NO_PERMISSION" {
		t.Fatalf("expected E_NO_PERMISSION, got code=%q events=%v", got, obsM.Events)
	}

	h.ClearAgentEventsFor(leader)
	obs = h.StepFor(leader, []protocol.InstantReq{{
		ID:     "I_withdraw_plank",
		Type:   "ORG_WITHDRAW",
		OrgID:  orgID,
		ItemID: "PLANK",
		Count:  taxPerSide,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_withdraw_plank"); got != "" {
		t.Fatalf("withdraw plank expected ok, got code=%q events=%v", got, obs.Events)
	}
	if got := invCount(h.LastObsFor(leader).Inventory, "PLANK"); got != taxPerSide {
		t.Fatalf("leader plank after withdraw: got %d want %d", got, taxPerSide)
	}

	h.ClearAgentEventsFor(leader)
	obs = h.StepFor(leader, []protocol.InstantReq{{
		ID:     "I_withdraw_iron",
		Type:   "ORG_WITHDRAW",
		OrgID:  orgID,
		ItemID: "IRON_INGOT",
		Count:  taxPerSide,
	}}, nil, nil)
	if got := actionResultCode(obs, "I_withdraw_iron"); got != "" {
		t.Fatalf("withdraw iron expected ok, got code=%q events=%v", got, obs.Events)
	}
	if got := invCount(h.LastObsFor(leader).Inventory, "IRON_INGOT"); got != recv+taxPerSide {
		t.Fatalf("leader iron after withdraw: got %d want %d", got, recv+taxPerSide)
	}
}

type auditSink struct {
	entries []world.AuditEntry
}

func (s *auditSink) WriteAudit(e world.AuditEntry) error {
	s.entries = append(s.entries, e)
	return nil
}

func (s *auditSink) lastTradeTaxRate() interface{} {
	for i := len(s.entries) - 1; i >= 0; i-- {
		e := s.entries[i]
		if e.Action != "TRADE" || e.Reason != "ACCEPT_TRADE" || e.Details == nil {
			continue
		}
		if v, ok := e.Details["tax_rate"]; ok {
			return v
		}
		return nil
	}
	return nil
}

func calcTax(count int, rate interface{}) int {
	f, ok := rate.(float64)
	if !ok {
		return 0
	}
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	tax := int(float64(count) * f)
	if tax < 0 {
		return 0
	}
	if tax > count {
		return count
	}
	return tax
}
