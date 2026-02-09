package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestOrgs_CreateJoinDeedAndTax(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     64,
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

	leader := join("leader")
	member := join("member")

	// Create org.
	w.applyInstant(leader, protocol.InstantReq{
		ID:      "I_create",
		Type:    "CREATE_ORG",
		OrgKind: "GUILD",
		OrgName: "RiverGuild",
	}, 0)
	if leader.OrgID == "" {
		t.Fatalf("expected leader org_id set")
	}
	org := w.orgs[leader.OrgID]
	if org == nil {
		t.Fatalf("expected org created")
	}
	if org.Members[leader.ID] != OrgLeader {
		t.Fatalf("expected leader role")
	}

	// Join org.
	w.applyInstant(member, protocol.InstantReq{
		ID:    "I_join",
		Type:  "JOIN_ORG",
		OrgID: org.OrgID,
	}, 1)
	if member.OrgID != org.OrgID {
		t.Fatalf("expected member joined org")
	}
	if org.Members[member.ID] != OrgMember {
		t.Fatalf("expected member role")
	}

	// Create a land and deed it to org.
	land := &LandClaim{
		LandID: "LAND_1",
		Owner:  leader.ID,
		Anchor: leader.Pos,
		Radius: 32,
		Flags:  ClaimFlags{AllowBuild: false, AllowBreak: false, AllowDamage: false, AllowTrade: false},
		// Tax should go to org treasury once deeded.
		MarketTax: 0.10,
	}
	w.claims[land.LandID] = land

	w.applyInstant(leader, protocol.InstantReq{
		ID:       "I_deed",
		Type:     "DEED_LAND",
		LandID:   land.LandID,
		NewOwner: org.OrgID,
	}, 2)
	if land.Owner != org.OrgID {
		t.Fatalf("expected land owner updated to org")
	}

	// Member should have member permissions on org-owned land.
	_, perms := w.permissionsFor(member.ID, leader.Pos)
	if !perms["can_build"] || !perms["can_break"] || !perms["can_trade"] {
		t.Fatalf("expected org member perms allowed, got %+v", perms)
	}

	// Tax should land in org treasury.
	leader.Inventory = map[string]int{"PLANK": 10}
	member.Inventory = map[string]int{"IRON_INGOT": 10}
	org.Treasury = map[string]int{}

	// Place both inside the land.
	member.Pos = leader.Pos

	// Offer and accept trade.
	w.applyInstant(leader, protocol.InstantReq{
		ID:      "I_offer",
		Type:    "OFFER_TRADE",
		To:      member.ID,
		Offer:   [][]interface{}{{"PLANK", 10}},
		Request: [][]interface{}{{"IRON_INGOT", 10}},
	}, 3)
	var tradeID string
	for id := range w.trades {
		tradeID = id
		break
	}
	if tradeID == "" {
		t.Fatalf("expected trade")
	}
	w.applyInstant(member, protocol.InstantReq{
		ID:      "I_accept",
		Type:    "ACCEPT_TRADE",
		TradeID: tradeID,
	}, 4)

	if got := leader.Inventory["IRON_INGOT"]; got != 9 {
		t.Fatalf("leader iron: got %d want 9", got)
	}
	if got := member.Inventory["PLANK"]; got != 9 {
		t.Fatalf("member plank: got %d want 9", got)
	}
	if got := org.Treasury["IRON_INGOT"]; got != 1 {
		t.Fatalf("org iron tax: got %d want 1", got)
	}
	if got := org.Treasury["PLANK"]; got != 1 {
		t.Fatalf("org plank tax: got %d want 1", got)
	}

	// Org treasury withdraw: only admins.
	w.applyInstant(member, protocol.InstantReq{
		ID:     "I_withdraw_bad",
		Type:   "ORG_WITHDRAW",
		OrgID:  org.OrgID,
		ItemID: "PLANK",
		Count:  1,
	}, 5)
	if got := org.Treasury["PLANK"]; got != 1 {
		t.Fatalf("treasury changed on unauthorized withdraw: got %d want 1", got)
	}

	w.applyInstant(leader, protocol.InstantReq{
		ID:     "I_withdraw_ok",
		Type:   "ORG_WITHDRAW",
		OrgID:  org.OrgID,
		ItemID: "PLANK",
		Count:  1,
	}, 6)
	if got := org.Treasury["PLANK"]; got != 0 {
		t.Fatalf("treasury plank after withdraw: got %d want 0", got)
	}
	if got := leader.Inventory["PLANK"]; got != 1 {
		t.Fatalf("leader plank after withdraw: got %d want 1", got)
	}
}
