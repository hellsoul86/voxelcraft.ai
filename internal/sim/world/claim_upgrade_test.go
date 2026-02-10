package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestClaimUpgrade_OwnerHappyPath(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{ID: "test", Seed: 1}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	// Join one agent.
	respCh := make(chan JoinResponse, 1)
	w.step([]JoinRequest{{Name: "owner", Resp: respCh}}, nil, nil)
	jr := <-respCh
	a := w.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	// Exactly enough for claim (1+1) + upgrade 32->64 (1+2).
	a.Inventory["BATTERY"] = 2
	a.Inventory["CRYSTAL_SHARD"] = 3

	y := w.cfg.Height - 2
	anchor := Vec3i{X: 0, Y: y, Z: 0}

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
	if got := w.claims[landID].Radius; got != 32 {
		t.Fatalf("radius=%d want 32", got)
	}

	// Upgrade.
	a.Events = nil
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         a.ID,
		Instants:        []protocol.InstantReq{{ID: "I1", Type: "UPGRADE_CLAIM", LandID: landID, Radius: 64}},
	}}})

	if got := w.claims[landID].Radius; got != 64 {
		t.Fatalf("radius=%d want 64", got)
	}
	if a.Inventory["BATTERY"] != 0 || a.Inventory["CRYSTAL_SHARD"] != 0 {
		t.Fatalf("expected upgrade to consume materials; inv=%v", a.Inventory)
	}

	foundOK := false
	for _, ev := range a.Events {
		if ev["type"] == "ACTION_RESULT" && ev["ref"] == "I1" {
			if ok, _ := ev["ok"].(bool); !ok {
				t.Fatalf("upgrade result not ok: %v", ev)
			}
			foundOK = true
		}
	}
	if !foundOK {
		t.Fatalf("missing ACTION_RESULT for upgrade: %v", a.Events)
	}
}

func TestClaimUpgrade_RequiresAdminAndNoOverlap(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{ID: "test", Seed: 2}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	respCh := make(chan JoinResponse, 2)
	w.step([]JoinRequest{{Name: "a1", Resp: respCh}, {Name: "a2", Resp: respCh}}, nil, nil)
	jr1 := <-respCh
	jr2 := <-respCh
	a1 := w.agents[jr1.Welcome.AgentID]
	a2 := w.agents[jr2.Welcome.AgentID]
	if a1 == nil || a2 == nil {
		t.Fatalf("missing agents")
	}

	// Resources for two claims + one upgrade attempt.
	a1.Inventory["BATTERY"] = 10
	a1.Inventory["CRYSTAL_SHARD"] = 10
	a2.Inventory["BATTERY"] = 10
	a2.Inventory["CRYSTAL_SHARD"] = 10

	y := w.cfg.Height - 2
	anchor1 := Vec3i{X: 0, Y: y, Z: 0}
	anchor2 := Vec3i{X: 80, Y: y, Z: 0} // does not overlap at radius=32, but overlaps if anchor1 upgrades to 64

	// Claim 1 (a1).
	w.step(nil, nil, []ActionEnvelope{{AgentID: a1.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         a1.ID,
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "CLAIM_LAND", Anchor: anchor1.ToArray(), Radius: 32}},
	}}})
	var land1 string
	for id, c := range w.claims {
		if c != nil && c.Owner == a1.ID {
			land1 = id
		}
	}
	if land1 == "" {
		t.Fatalf("missing land1 id")
	}

	// Claim 2 (a2).
	w.step(nil, nil, []ActionEnvelope{{AgentID: a2.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         a2.ID,
		Tasks:           []protocol.TaskReq{{ID: "K2", Type: "CLAIM_LAND", Anchor: anchor2.ToArray(), Radius: 32}},
	}}})
	if len(w.claims) != 2 {
		t.Fatalf("claims=%d want 2", len(w.claims))
	}

	// Non-admin (a2) cannot upgrade a1's claim.
	a2.Events = nil
	w.step(nil, nil, []ActionEnvelope{{AgentID: a2.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         a2.ID,
		Instants:        []protocol.InstantReq{{ID: "I1", Type: "UPGRADE_CLAIM", LandID: land1, Radius: 64}},
	}}})
	if got := w.claims[land1].Radius; got != 32 {
		t.Fatalf("radius changed unexpectedly: %d", got)
	}
	assertLastActionResultCode(t, a2, "I1", "E_NO_PERMISSION")

	// Admin upgrade should fail due to overlap with land2.
	a1.Events = nil
	w.step(nil, nil, []ActionEnvelope{{AgentID: a1.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         a1.ID,
		Instants:        []protocol.InstantReq{{ID: "I2", Type: "UPGRADE_CLAIM", LandID: land1, Radius: 64}},
	}}})
	if got := w.claims[land1].Radius; got != 32 {
		t.Fatalf("radius=%d want 32 (upgrade should fail due to overlap)", got)
	}
	assertLastActionResultCode(t, a1, "I2", "E_CONFLICT")
}

func TestClaimUpgrade_BlockedByMaintenanceStageAndMaterials(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{ID: "test", Seed: 3}, cats)
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

	a.Inventory["BATTERY"] = 1
	a.Inventory["CRYSTAL_SHARD"] = 1

	y := w.cfg.Height - 2
	anchor := Vec3i{X: 0, Y: y, Z: 0}

	// Claim land.
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         a.ID,
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "CLAIM_LAND", Anchor: anchor.ToArray(), Radius: 32}},
	}}})
	var landID string
	for id := range w.claims {
		landID = id
	}
	if landID == "" {
		t.Fatalf("missing land id")
	}

	// Missing materials for upgrade.
	a.Events = nil
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         a.ID,
		Instants:        []protocol.InstantReq{{ID: "I1", Type: "UPGRADE_CLAIM", LandID: landID, Radius: 64}},
	}}})
	assertLastActionResultCode(t, a, "I1", "E_NO_RESOURCE")

	// Put materials in inventory, but maintenance stage prevents expansion.
	a.Inventory["BATTERY"] = 1
	a.Inventory["CRYSTAL_SHARD"] = 2
	w.claims[landID].MaintenanceStage = 1
	a.Events = nil
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         a.ID,
		Instants:        []protocol.InstantReq{{ID: "I2", Type: "UPGRADE_CLAIM", LandID: landID, Radius: 64}},
	}}})
	assertLastActionResultCode(t, a, "I2", "E_NO_PERMISSION")
}

func assertLastActionResultCode(t *testing.T, a *Agent, ref string, wantCode string) {
	t.Helper()
	for i := len(a.Events) - 1; i >= 0; i-- {
		ev := a.Events[i]
		if ev["type"] != "ACTION_RESULT" || ev["ref"] != ref {
			continue
		}
		if got, _ := ev["code"].(string); got != wantCode {
			t.Fatalf("ACTION_RESULT code=%q want %q (ev=%v)", got, wantCode, ev)
		}
		return
	}
	t.Fatalf("missing ACTION_RESULT for %s; events=%v", ref, a.Events)
}
