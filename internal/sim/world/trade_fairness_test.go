package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestTrade_FairTradeAwardsSocialFun(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{ID: "test", Seed: 11}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	resp := make(chan JoinResponse, 2)
	w.step([]JoinRequest{{Name: "a1", Resp: resp}, {Name: "a2", Resp: resp}}, nil, nil)
	j1 := <-resp
	j2 := <-resp
	a1 := w.agents[j1.Welcome.AgentID]
	a2 := w.agents[j2.Welcome.AgentID]
	if a1 == nil || a2 == nil {
		t.Fatalf("missing agents")
	}

	// Clear join novelty events to make assertions simpler.
	a1.Events = nil
	a2.Events = nil

	a1.Inventory["PLANK"] = 5
	a2.Inventory["IRON_INGOT"] = 1
	a2.Pos = a1.Pos

	w.applyInstant(a1, protocol.InstantReq{
		ID:      "I_offer",
		Type:    "OFFER_TRADE",
		To:      a2.ID,
		Offer:   [][]interface{}{{"PLANK", 5}},
		Request: [][]interface{}{{"IRON_INGOT", 1}},
	}, 0)

	tradeID := ""
	for id := range w.trades {
		tradeID = id
		break
	}
	if tradeID == "" {
		t.Fatalf("expected trade id")
	}

	w.applyInstant(a2, protocol.InstantReq{
		ID:      "I_accept",
		Type:    "ACCEPT_TRADE",
		TradeID: tradeID,
	}, 1)

	if a1.RepSocial != 501 || a2.RepSocial != 501 {
		t.Fatalf("repSocial a1=%d a2=%d want 501", a1.RepSocial, a2.RepSocial)
	}
	if a1.Fun.Social <= 0 || a2.Fun.Social <= 0 {
		t.Fatalf("expected social fun to increase; a1=%d a2=%d", a1.Fun.Social, a2.Fun.Social)
	}
}

func TestTrade_UnfairTradeDoesNotAwardSocialFun(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{ID: "test", Seed: 12}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	resp := make(chan JoinResponse, 2)
	w.step([]JoinRequest{{Name: "a1", Resp: resp}, {Name: "a2", Resp: resp}}, nil, nil)
	j1 := <-resp
	j2 := <-resp
	a1 := w.agents[j1.Welcome.AgentID]
	a2 := w.agents[j2.Welcome.AgentID]
	if a1 == nil || a2 == nil {
		t.Fatalf("missing agents")
	}

	a1.Events = nil
	a2.Events = nil

	a1.Inventory["PLANK"] = 1
	a2.Inventory["CRYSTAL_SHARD"] = 1
	a2.Pos = a1.Pos

	w.applyInstant(a1, protocol.InstantReq{
		ID:      "I_offer",
		Type:    "OFFER_TRADE",
		To:      a2.ID,
		Offer:   [][]interface{}{{"PLANK", 1}},
		Request: [][]interface{}{{"CRYSTAL_SHARD", 1}},
	}, 0)

	tradeID := ""
	for id := range w.trades {
		tradeID = id
		break
	}
	if tradeID == "" {
		t.Fatalf("expected trade id")
	}

	w.applyInstant(a2, protocol.InstantReq{
		ID:      "I_accept",
		Type:    "ACCEPT_TRADE",
		TradeID: tradeID,
	}, 1)

	if a1.RepSocial != 500 || a2.RepSocial != 500 {
		t.Fatalf("repSocial a1=%d a2=%d want 500", a1.RepSocial, a2.RepSocial)
	}
	if a1.Fun.Social != 0 || a2.Fun.Social != 0 {
		t.Fatalf("expected no social fun; a1=%d a2=%d", a1.Fun.Social, a2.Fun.Social)
	}
}
