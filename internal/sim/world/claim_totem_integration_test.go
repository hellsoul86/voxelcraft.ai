package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestClaimTotem_MiningRemovesClaimAndBoundLaws(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{ID: "test", Seed: 4}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	respCh := make(chan JoinResponse, 1)
	w.step([]JoinRequest{{Name: "owner", Resp: respCh}}, nil, nil)
	jr := <-respCh
	a := w.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	a.Inventory["BATTERY"] = 3
	a.Inventory["CRYSTAL_SHARD"] = 3

	anchor := a.Pos // spawn is already on an AIR block

	// Claim land.
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         a.ID,
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "CLAIM_LAND", Anchor: anchor.ToArray(), Radius: 32}},
	}}})
	if len(w.claims) != 1 {
		t.Fatalf("claims=%d want 1", len(w.claims))
	}
	var landID string
	for id := range w.claims {
		landID = id
	}
	if landID == "" {
		t.Fatalf("missing land id")
	}

	// Add a bound law (directly) and ensure it gets removed when the claim is removed.
	w.laws["L1"] = &Law{LawID: "L1", LandID: landID, TemplateID: "MARKET_TAX", Title: "t", Params: map[string]string{"rate": "0.05"}, Status: LawActive}

	// Mine the claim totem block at anchor.
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         a.ID,
		Tasks:           []protocol.TaskReq{{ID: "K2", Type: "MINE", BlockPos: anchor.ToArray()}},
	}}})

	// Complete mining (10 work ticks total; one happens in the start tick).
	for i := 0; i < 12; i++ {
		w.step(nil, nil, nil)
	}

	if len(w.claims) != 0 {
		t.Fatalf("expected claim removed; claims=%v", w.claims)
	}
	if len(w.laws) != 0 {
		t.Fatalf("expected bound laws removed; laws=%v", w.laws)
	}
	if w.chunks.GetBlock(anchor) != w.chunks.gen.Air {
		t.Fatalf("expected anchor block to be AIR after mining")
	}

	// Ensure re-claim works after totem removal.
	a.Events = nil
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         a.ID,
		Tasks:           []protocol.TaskReq{{ID: "K3", Type: "CLAIM_LAND", Anchor: anchor.ToArray(), Radius: 32}},
	}}})
	if len(w.claims) != 1 {
		t.Fatalf("expected re-claim to succeed; claims=%d", len(w.claims))
	}
}
