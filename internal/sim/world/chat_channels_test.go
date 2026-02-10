package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestChat_CITY_OrgOnly(t *testing.T) {
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
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: name, DeltaVoxels: false, Out: nil, Resp: resp})
		j := <-resp
		return w.agents[j.Welcome.AgentID]
	}

	leader := join("leader")
	member := join("member")
	outsider := join("outsider")
	if leader == nil || member == nil || outsider == nil {
		t.Fatalf("missing agents")
	}

	w.applyInstant(leader, protocol.InstantReq{
		ID:      "I_create",
		Type:    "CREATE_ORG",
		OrgKind: "CITY",
		OrgName: "TestCity",
	}, 0)
	if leader.OrgID == "" {
		t.Fatalf("expected org created")
	}
	orgID := leader.OrgID

	w.applyInstant(member, protocol.InstantReq{
		ID:    "I_join",
		Type:  "JOIN_ORG",
		OrgID: orgID,
	}, 1)
	if member.OrgID != orgID {
		t.Fatalf("expected member joined org")
	}

	leader.Events = nil
	member.Events = nil
	outsider.Events = nil

	w.applyInstant(leader, protocol.InstantReq{
		ID:      "I_city",
		Type:    "SAY",
		Channel: "CITY",
		Text:    "hello city",
	}, 2)

	if got := countChatEvents(leader, "CITY"); got != 1 {
		t.Fatalf("leader CITY chat events=%d want 1", got)
	}
	if got := countChatEvents(member, "CITY"); got != 1 {
		t.Fatalf("member CITY chat events=%d want 1", got)
	}
	if got := countChatEvents(outsider, "CITY"); got != 0 {
		t.Fatalf("outsider CITY chat events=%d want 0", got)
	}

	outsider.Events = nil
	w.applyInstant(outsider, protocol.InstantReq{
		ID:      "I_city2",
		Type:    "SAY",
		Channel: "CITY",
		Text:    "hi",
	}, 3)
	if got := actionResultCode(outsider, "I_city2"); got != "E_NO_PERMISSION" {
		t.Fatalf("outsider CITY result code=%q want %q", got, "E_NO_PERMISSION")
	}
}

func TestChat_MARKET_RequiresCanTrade(t *testing.T) {
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
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: name, DeltaVoxels: false, Out: nil, Resp: resp})
		j := <-resp
		return w.agents[j.Welcome.AgentID]
	}
	owner := join("owner")
	visitor := join("visitor")
	if owner == nil || visitor == nil {
		t.Fatalf("missing agents")
	}

	land := &LandClaim{
		LandID: "LAND_MARKET_CHAT",
		Owner:  owner.ID,
		Anchor: owner.Pos,
		Radius: 32,
		Flags:  ClaimFlags{AllowBuild: true, AllowBreak: true, AllowDamage: false, AllowTrade: false},
	}
	w.claims[land.LandID] = land

	visitor.Pos = owner.Pos
	visitor.Events = nil
	w.applyInstant(visitor, protocol.InstantReq{
		ID:      "I_m1",
		Type:    "SAY",
		Channel: "MARKET",
		Text:    "buy iron",
	}, 0)
	if got := actionResultCode(visitor, "I_m1"); got != "E_NO_PERMISSION" {
		t.Fatalf("visitor MARKET result code=%q want %q", got, "E_NO_PERMISSION")
	}

	// Move visitor outside the claim: MARKET chat should be allowed in the wild.
	visitor.Pos = Vec3i{X: owner.Pos.X + land.Radius + 10, Y: owner.Pos.Y, Z: owner.Pos.Z}
	visitor.Events = nil
	w.applyInstant(visitor, protocol.InstantReq{
		ID:      "I_m2",
		Type:    "SAY",
		Channel: "MARKET",
		Text:    "sell planks",
	}, 1)
	if got := actionResultCode(visitor, "I_m2"); got != "" {
		t.Fatalf("visitor MARKET expected ok, got code=%q", got)
	}
	if got := countChatEvents(visitor, "MARKET"); got != 1 {
		t.Fatalf("visitor MARKET chat events=%d want 1", got)
	}
}

func TestChat_MARKET_UsesSeparateRateLimit(t *testing.T) {
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

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "bot", DeltaVoxels: false, Out: nil, Resp: resp})
	j := <-resp
	a := w.agents[j.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	// Make LOCAL chat stricter than MARKET chat to verify separate buckets.
	w.cfg.RateLimits.SayWindowTicks = 50
	w.cfg.RateLimits.SayMax = 1
	w.cfg.RateLimits.MarketSayWindowTicks = 50
	w.cfg.RateLimits.MarketSayMax = 2

	a.Events = nil
	w.applyInstant(a, protocol.InstantReq{ID: "I_l1", Type: "SAY", Channel: "LOCAL", Text: "l1"}, 0)
	w.applyInstant(a, protocol.InstantReq{ID: "I_l2", Type: "SAY", Channel: "LOCAL", Text: "l2"}, 0)
	if got := actionResultCode(a, "I_l2"); got != "E_RATE_LIMIT" {
		t.Fatalf("second LOCAL expected rate limit, got code=%q", got)
	}

	a.Events = nil
	w.applyInstant(a, protocol.InstantReq{ID: "I_m1", Type: "SAY", Channel: "MARKET", Text: "m1"}, 0)
	w.applyInstant(a, protocol.InstantReq{ID: "I_m2", Type: "SAY", Channel: "MARKET", Text: "m2"}, 0)
	w.applyInstant(a, protocol.InstantReq{ID: "I_m3", Type: "SAY", Channel: "MARKET", Text: "m3"}, 0)
	if got := actionResultCode(a, "I_m3"); got != "E_RATE_LIMIT" {
		t.Fatalf("third MARKET expected rate limit, got code=%q", got)
	}
	if got := countChatEvents(a, "MARKET"); got != 2 {
		t.Fatalf("MARKET chat events=%d want 2", got)
	}
}

func countChatEvents(a *Agent, channel string) int {
	if a == nil {
		return 0
	}
	n := 0
	for _, e := range a.Events {
		if typ, _ := e["type"].(string); typ != "CHAT" {
			continue
		}
		if ch, _ := e["channel"].(string); ch == channel {
			n++
		}
	}
	return n
}

func actionResultCode(a *Agent, ref string) string {
	if a == nil {
		return ""
	}
	for _, e := range a.Events {
		if typ, _ := e["type"].(string); typ != "ACTION_RESULT" {
			continue
		}
		if r, _ := e["ref"].(string); r != ref {
			continue
		}
		if code, ok := e["code"].(string); ok {
			return code
		}
		return ""
	}
	return ""
}
