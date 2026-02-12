package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestWanted_TagAndCityCoreBlock(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{ID: "test", Seed: 5}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	respCh := make(chan JoinResponse, 2)
	w.step([]JoinRequest{{Name: "mayor", Resp: respCh}, {Name: "criminal", Resp: respCh}}, nil, nil)
	jrMayor := <-respCh
	jrCrim := <-respCh
	mayor := w.agents[jrMayor.Welcome.AgentID]
	crim := w.agents[jrCrim.Welcome.AgentID]
	if mayor == nil || crim == nil {
		t.Fatalf("missing agents")
	}

	// Create a CITY org.
	mayor.Events = nil
	w.step(nil, nil, []ActionEnvelope{{AgentID: mayor.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         mayor.ID,
		Instants:        []protocol.InstantReq{{ID: "I1", Type: "CREATE_ORG", OrgKind: "CITY", OrgName: "TestCity"}},
	}}})
	orgID := ""
	for _, ev := range mayor.Events {
		if ev["type"] == "ACTION_RESULT" && ev["ref"] == "I1" {
			if s, ok := ev["org_id"].(string); ok && s != "" {
				orgID = s
			}
		}
	}
	if orgID == "" {
		t.Fatalf("missing org_id from CREATE_ORG: %v", mayor.Events)
	}

	// Claim a small land parcel near the mayor and deed it to the CITY org.
	mayor.Inventory["BATTERY"] = 2
	mayor.Inventory["CRYSTAL_SHARD"] = 2
	anchor := mayor.Pos

	w.step(nil, nil, []ActionEnvelope{{AgentID: mayor.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         mayor.ID,
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "CLAIM_LAND", Anchor: anchor.ToArray(), Radius: 8}},
	}}})
	landID := ""
	for id, c := range w.claims {
		if c != nil && c.Owner == mayor.ID {
			landID = id
		}
	}
	if landID == "" {
		t.Fatalf("missing land id")
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: mayor.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         mayor.ID,
		Instants:        []protocol.InstantReq{{ID: "I2", Type: "DEED_LAND", LandID: landID, NewOwner: orgID}},
	}}})
	if w.claims[landID].Owner != orgID {
		t.Fatalf("expected land owner deeded to org; got %q want %q", w.claims[landID].Owner, orgID)
	}

	// Place the criminal just outside the core (core radius == claim radius here).
	startX := anchor.X + 9 // outside claim/core (radius=8), but within OBS entity radius 16
	crim.Pos = Vec3i{X: startX, Y: w.surfaceY(startX, anchor.Z), Z: anchor.Z}
	crim.RepLaw = 100 // wanted

	// Mayor should see the criminal as "wanted".
	obs := w.buildObs(mayor, &clientState{}, w.CurrentTick())
	foundWanted := false
	for _, e := range obs.Entities {
		if e.Type != "AGENT" || e.ID != crim.ID {
			continue
		}
		for _, tag := range e.Tags {
			if tag == "wanted" {
				foundWanted = true
				break
			}
		}
	}
	if !foundWanted {
		t.Fatalf("expected wanted tag in OBS entities")
	}

	// Movement into CITY core should be blocked for wanted agents.
	crim.Events = nil
	w.step(nil, nil, []ActionEnvelope{{AgentID: crim.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         crim.ID,
		Tasks:           []protocol.TaskReq{{ID: "K2", Type: "MOVE_TO", Target: anchor.ToArray(), Tolerance: 1.2}},
	}}})
	if crim.MoveTask != nil {
		t.Fatalf("expected move task to fail and be cleared")
	}
	foundFail := false
	for _, ev := range crim.Events {
		if ev["type"] == "TASK_FAIL" && ev["code"] == "E_NO_PERMISSION" {
			foundFail = true
			break
		}
	}
	if !foundFail {
		t.Fatalf("expected TASK_FAIL E_NO_PERMISSION; events=%v", crim.Events)
	}
}

func TestCityCore_AllowsNonWanted(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{ID: "test", Seed: 6}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	respCh := make(chan JoinResponse, 2)
	w.step([]JoinRequest{{Name: "mayor", Resp: respCh}, {Name: "visitor", Resp: respCh}}, nil, nil)
	jrMayor := <-respCh
	jrVis := <-respCh
	mayor := w.agents[jrMayor.Welcome.AgentID]
	vis := w.agents[jrVis.Welcome.AgentID]
	if mayor == nil || vis == nil {
		t.Fatalf("missing agents")
	}

	// CITY org + deeded claim.
	mayor.Events = nil
	w.step(nil, nil, []ActionEnvelope{{AgentID: mayor.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         mayor.ID,
		Instants:        []protocol.InstantReq{{ID: "I1", Type: "CREATE_ORG", OrgKind: "CITY", OrgName: "TestCity"}},
	}}})
	orgID := ""
	for _, ev := range mayor.Events {
		if ev["type"] == "ACTION_RESULT" && ev["ref"] == "I1" {
			orgID, _ = ev["org_id"].(string)
		}
	}
	if orgID == "" {
		t.Fatalf("missing org_id")
	}

	mayor.Inventory["BATTERY"] = 2
	mayor.Inventory["CRYSTAL_SHARD"] = 2
	anchor := mayor.Pos
	w.step(nil, nil, []ActionEnvelope{{AgentID: mayor.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         mayor.ID,
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "CLAIM_LAND", Anchor: anchor.ToArray(), Radius: 8}},
	}}})
	landID := ""
	for id, c := range w.claims {
		if c != nil && c.Owner == mayor.ID {
			landID = id
		}
	}
	if landID == "" {
		t.Fatalf("missing land id")
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: mayor.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         mayor.ID,
		Instants:        []protocol.InstantReq{{ID: "I2", Type: "DEED_LAND", LandID: landID, NewOwner: orgID}},
	}}})

	// Visitor outside claim, but not wanted -> should be able to step into core.
	startX := anchor.X + 9
	vis.Pos = Vec3i{X: startX, Y: w.surfaceY(startX, anchor.Z), Z: anchor.Z}
	vis.RepLaw = 500
	for x := anchor.X; x <= startX; x++ {
		setAir(w, Vec3i{X: x, Y: 0, Z: anchor.Z})
	}

	w.step(nil, nil, []ActionEnvelope{{AgentID: vis.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         vis.ID,
		Tasks:           []protocol.TaskReq{{ID: "K2", Type: "MOVE_TO", Target: anchor.ToArray(), Tolerance: 1.2}},
	}}})

	if vis.Pos.X != startX-1 {
		t.Fatalf("expected visitor to move one step toward core; pos=%+v", vis.Pos)
	}
}
