package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestChat_CITY_OrgOnly(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:         "test",
		WorldType:  "CITY_HUB",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       1,
		BoundaryR:  4000,
	}, cats, "leader")
	leader := h.DefaultAgentID
	member := h.Join("member")
	outsider := h.Join("outsider")

	h.StepFor(leader, []protocol.InstantReq{{
		ID:      "I_create",
		Type:    "CREATE_ORG",
		OrgKind: "CITY",
		OrgName: "TestCity",
	}}, nil, nil)
	orgID := actionResultFieldString(h.LastObsFor(leader), "I_create", "org_id")
	if orgID == "" {
		t.Fatalf("expected org_id in ACTION_RESULT")
	}

	h.StepFor(member, []protocol.InstantReq{{
		ID:    "I_join",
		Type:  "JOIN_ORG",
		OrgID: orgID,
	}}, nil, nil)
	if got := actionResultCode(h.LastObsFor(member), "I_join"); got != "" {
		t.Fatalf("member join expected ok, got code=%q", got)
	}

	h.ClearAgentEventsFor(leader)
	h.ClearAgentEventsFor(member)
	h.ClearAgentEventsFor(outsider)

	h.StepFor(leader, []protocol.InstantReq{{
		ID:      "I_city",
		Type:    "SAY",
		Channel: "CITY",
		Text:    "hello city",
	}}, nil, nil)

	if got := countChatEvents(h.LastObsFor(leader), "CITY"); got != 1 {
		t.Fatalf("leader CITY chat events=%d want 1", got)
	}
	if got := countChatEvents(h.LastObsFor(member), "CITY"); got != 1 {
		t.Fatalf("member CITY chat events=%d want 1", got)
	}
	if got := countChatEvents(h.LastObsFor(outsider), "CITY"); got != 0 {
		t.Fatalf("outsider CITY chat events=%d want 0", got)
	}

	h.ClearAgentEventsFor(outsider)
	h.StepFor(outsider, []protocol.InstantReq{{
		ID:      "I_city2",
		Type:    "SAY",
		Channel: "CITY",
		Text:    "hi",
	}}, nil, nil)
	if got := actionResultCode(h.LastObsFor(outsider), "I_city2"); got != "E_NO_PERMISSION" {
		t.Fatalf("outsider CITY result code=%q want %q", got, "E_NO_PERMISSION")
	}
}

func TestChat_MARKET_RequiresCanTrade(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:         "test",
		WorldType:  "OVERWORLD",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       1,
		BoundaryR:  4000,
		StarterItems: map[string]int{
			"BATTERY":      5,
			"CRYSTAL_SHARD": 5,
		},
	}, cats, "owner")
	owner := h.DefaultAgentID
	visitor := h.Join("visitor")

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")

	h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:     "K1",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 32,
	}}, nil)
	if got := actionResultCode(h.LastObsFor(owner), "K1"); got != "" {
		t.Fatalf("claim expected ok, got code=%q", got)
	}

	h.SetAgentPosFor(visitor, anchor)
	h.ClearAgentEventsFor(visitor)
	h.StepFor(visitor, []protocol.InstantReq{{
		ID:      "I_m1",
		Type:    "SAY",
		Channel: "MARKET",
		Text:    "buy iron",
	}}, nil, nil)
	if got := actionResultCode(h.LastObsFor(visitor), "I_m1"); got != "E_NO_PERMISSION" {
		t.Fatalf("visitor MARKET result code=%q want %q", got, "E_NO_PERMISSION")
	}

	// Move visitor outside the claim: MARKET chat should be allowed in the wild.
	h.SetAgentPosFor(visitor, world.Vec3i{X: anchor.X + 32 + 10, Y: 0, Z: anchor.Z})
	h.ClearAgentEventsFor(visitor)
	h.StepFor(visitor, []protocol.InstantReq{{
		ID:      "I_m2",
		Type:    "SAY",
		Channel: "MARKET",
		Text:    "sell planks",
	}}, nil, nil)
	if got := actionResultCode(h.LastObsFor(visitor), "I_m2"); got != "" {
		t.Fatalf("visitor MARKET expected ok, got code=%q", got)
	}
	if got := countChatEvents(h.LastObsFor(visitor), "MARKET"); got != 1 {
		t.Fatalf("visitor MARKET chat events=%d want 1", got)
	}
}

func TestChat_MARKET_UsesSeparateRateLimit(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       1,
		BoundaryR:  4000,
		RateLimits: world.RateLimitConfig{
			SayWindowTicks:       50,
			SayMax:               1,
			MarketSayWindowTicks: 50,
			MarketSayMax:         2,
			WhisperWindowTicks:   50,
			WhisperMax:           5,
			OfferTradeWindowTicks: 50,
			OfferTradeMax:         3,
			PostBoardWindowTicks:  600,
			PostBoardMax:          1,
		},
	}, cats, "bot")

	h.ClearAgentEvents()
	h.Step([]protocol.InstantReq{
		{ID: "I_l1", Type: "SAY", Channel: "LOCAL", Text: "l1"},
		{ID: "I_l2", Type: "SAY", Channel: "LOCAL", Text: "l2"},
	}, nil, nil)
	if got := actionResultCode(h.LastObs(), "I_l2"); got != "E_RATE_LIMIT" {
		t.Fatalf("second LOCAL expected rate limit, got code=%q", got)
	}

	h.ClearAgentEvents()
	h.Step([]protocol.InstantReq{
		{ID: "I_m1", Type: "SAY", Channel: "MARKET", Text: "m1"},
		{ID: "I_m2", Type: "SAY", Channel: "MARKET", Text: "m2"},
		{ID: "I_m3", Type: "SAY", Channel: "MARKET", Text: "m3"},
	}, nil, nil)
	if got := actionResultCode(h.LastObs(), "I_m3"); got != "E_RATE_LIMIT" {
		t.Fatalf("third MARKET expected rate limit, got code=%q", got)
	}
	if got := countChatEvents(h.LastObs(), "MARKET"); got != 2 {
		t.Fatalf("MARKET chat events=%d want 2", got)
	}
}

func countChatEvents(obs protocol.ObsMsg, channel string) int {
	n := 0
	for _, e := range obs.Events {
		if typ, _ := e["type"].(string); typ != "CHAT" {
			continue
		}
		if ch, _ := e["channel"].(string); ch != channel {
			continue
		}
		n++
	}
	return n
}
