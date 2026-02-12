package worldtest

import (
	"math"
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestTrade_FairTradeAwardsSocialFun(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{ID: "test", Seed: 11}, cats, "a1")
	a1 := h.DefaultAgentID
	a2 := h.Join("a2")

	h.ClearAgentEventsFor(a1)
	h.ClearAgentEventsFor(a2)

	// Setup inventory and proximity for a P2P trade.
	h.AddInventoryFor(a1, "PLANK", 5)
	h.AddInventoryFor(a2, "IRON_INGOT", 1)
	p1 := h.LastObsFor(a1).Self.Pos
	h.SetAgentPosFor(a2, world.Vec3i{X: p1[0], Y: p1[1], Z: p1[2]})

	before1 := h.LastObsFor(a1).Self.Reputation.Social
	before2 := h.LastObsFor(a2).Self.Reputation.Social

	obsOffer := h.StepFor(a1, []protocol.InstantReq{{
		ID:      "I_offer",
		Type:    "OFFER_TRADE",
		To:      a2,
		Offer:   [][]interface{}{{"PLANK", 5}},
		Request: [][]interface{}{{"IRON_INGOT", 1}},
	}}, nil, nil)
	tradeID := actionResultFieldString(obsOffer, "I_offer", "trade_id")
	if tradeID == "" {
		t.Fatalf("expected trade_id in ACTION_RESULT for I_offer")
	}

	_ = h.StepFor(a2, []protocol.InstantReq{{
		ID:      "I_accept",
		Type:    "ACCEPT_TRADE",
		TradeID: tradeID,
	}}, nil, nil)

	after1 := h.LastObsFor(a1).Self.Reputation.Social
	after2 := h.LastObsFor(a2).Self.Reputation.Social

	if got, want := after1, before1+0.001; math.Abs(got-want) > 1e-9 {
		t.Fatalf("a1 social rep: got %0.6f want %0.6f", got, want)
	}
	if got, want := after2, before2+0.001; math.Abs(got-want) > 1e-9 {
		t.Fatalf("a2 social rep: got %0.6f want %0.6f", got, want)
	}

	obs1 := h.LastObsFor(a1)
	obs2 := h.LastObsFor(a2)
	if obs1.FunScore == nil || obs2.FunScore == nil {
		t.Fatalf("expected fun_score in OBS")
	}
	if obs1.FunScore.Social <= 0 || obs2.FunScore.Social <= 0 {
		t.Fatalf("expected social fun to increase; a1=%d a2=%d", obs1.FunScore.Social, obs2.FunScore.Social)
	}
}

func TestTrade_UnfairTradeDoesNotAwardSocialFun(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{ID: "test", Seed: 12}, cats, "a1")
	a1 := h.DefaultAgentID
	a2 := h.Join("a2")

	h.ClearAgentEventsFor(a1)
	h.ClearAgentEventsFor(a2)

	h.AddInventoryFor(a1, "PLANK", 1)
	h.AddInventoryFor(a2, "CRYSTAL_SHARD", 1)
	p1 := h.LastObsFor(a1).Self.Pos
	h.SetAgentPosFor(a2, world.Vec3i{X: p1[0], Y: p1[1], Z: p1[2]})

	before1 := h.LastObsFor(a1).Self.Reputation.Social
	before2 := h.LastObsFor(a2).Self.Reputation.Social

	obsOffer := h.StepFor(a1, []protocol.InstantReq{{
		ID:      "I_offer",
		Type:    "OFFER_TRADE",
		To:      a2,
		Offer:   [][]interface{}{{"PLANK", 1}},
		Request: [][]interface{}{{"CRYSTAL_SHARD", 1}},
	}}, nil, nil)
	tradeID := actionResultFieldString(obsOffer, "I_offer", "trade_id")
	if tradeID == "" {
		t.Fatalf("expected trade_id in ACTION_RESULT for I_offer")
	}

	_ = h.StepFor(a2, []protocol.InstantReq{{
		ID:      "I_accept",
		Type:    "ACCEPT_TRADE",
		TradeID: tradeID,
	}}, nil, nil)

	after1 := h.LastObsFor(a1).Self.Reputation.Social
	after2 := h.LastObsFor(a2).Self.Reputation.Social

	if got, want := after1, before1; math.Abs(got-want) > 1e-9 {
		t.Fatalf("a1 social rep: got %0.6f want %0.6f", got, want)
	}
	if got, want := after2, before2; math.Abs(got-want) > 1e-9 {
		t.Fatalf("a2 social rep: got %0.6f want %0.6f", got, want)
	}

	obs1 := h.LastObsFor(a1)
	obs2 := h.LastObsFor(a2)
	if obs1.FunScore == nil || obs2.FunScore == nil {
		t.Fatalf("expected fun_score in OBS")
	}
	if obs1.FunScore.Social != 0 || obs2.FunScore.Social != 0 {
		t.Fatalf("expected no social fun; a1=%d a2=%d", obs1.FunScore.Social, obs2.FunScore.Social)
	}
}
