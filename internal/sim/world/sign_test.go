package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestSign_SetAndOpenAndObsEntity(t *testing.T) {
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
		Seed:       42,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "writer", DeltaVoxels: false, Out: nil, Resp: resp})
	j := <-resp
	a := w.agents[j.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	// Place a sign block next to the agent.
	signPos := Vec3i{X: a.Pos.X + 1, Y: a.Pos.Y, Z: a.Pos.Z}
	sid := w.catalogs.Blocks.Index["SIGN"]
	w.chunks.SetBlock(signPos, sid)
	a.Pos = signPos

	w.applyInstant(a, protocol.InstantReq{
		ID:       "I_set",
		Type:     "SET_SIGN",
		TargetID: signIDAt(signPos),
		Text:     "Hello Sign",
	}, 0)

	s := w.signs[signPos]
	if s == nil || s.Text != "Hello Sign" || s.UpdatedBy != a.ID {
		t.Fatalf("unexpected sign state: %+v", s)
	}

	// Sign should appear as an entity with has_text tag.
	obs := w.buildObs(a, &clientState{DeltaVoxels: false}, 0)
	foundEnt := false
	for _, e := range obs.Entities {
		if e.Type == "SIGN" && e.ID == signIDAt(signPos) {
			foundEnt = true
			hasText := false
			for _, tag := range e.Tags {
				if tag == "has_text" {
					hasText = true
					break
				}
			}
			if !hasText {
				t.Fatalf("expected has_text tag on SIGN entity: %+v", e)
			}
			break
		}
	}
	if !foundEnt {
		t.Fatalf("expected SIGN entity in OBS")
	}

	// OPEN should return SIGN event.
	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         a.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K_open", Type: "OPEN", TargetID: signIDAt(signPos)},
		},
	}
	a.Events = nil
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})

	found := false
	for _, ev := range a.Events {
		if ev["type"] == "SIGN" && ev["sign_id"] == signIDAt(signPos) {
			if ev["text"] != "Hello Sign" {
				t.Fatalf("unexpected sign text: %+v", ev)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected SIGN event from OPEN")
	}
}

func TestSign_SetPermissionDeniedForVisitorWhenAllowBuildFalse(t *testing.T) {
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
		Seed:       42,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	join := func(name string) *Agent {
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: name, DeltaVoxels: false, Out: nil, Resp: resp})
		r := <-resp
		return w.agents[r.Welcome.AgentID]
	}
	owner := join("owner")
	visitor := join("visitor")
	if owner == nil || visitor == nil {
		t.Fatalf("missing agents")
	}

	landID := w.newLandID(owner.ID)
	w.claims[landID] = &LandClaim{
		LandID:  landID,
		Owner:   owner.ID,
		Anchor:  owner.Pos,
		Radius:  32,
		Flags:   ClaimFlags{AllowBuild: false, AllowBreak: false, AllowDamage: false, AllowTrade: true},
		Members: map[string]bool{},
	}

	signPos := owner.Pos
	sid := w.catalogs.Blocks.Index["SIGN"]
	w.chunks.SetBlock(signPos, sid)
	visitor.Pos = signPos

	w.applyInstant(visitor, protocol.InstantReq{
		ID:       "I_set",
		Type:     "SET_SIGN",
		TargetID: signIDAt(signPos),
		Text:     "nope",
	}, 0)

	if len(visitor.Events) == 0 {
		t.Fatalf("expected action result")
	}
	ev := visitor.Events[len(visitor.Events)-1]
	if ok, _ := ev["ok"].(bool); ok {
		t.Fatalf("expected denied: %+v", ev)
	}
	if code, _ := ev["code"].(string); code != "E_NO_PERMISSION" {
		t.Fatalf("expected E_NO_PERMISSION, got %+v", ev)
	}
}
