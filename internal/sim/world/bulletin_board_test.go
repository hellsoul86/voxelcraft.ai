package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestBulletinBoard_Physical_PostOpenAndPermissions(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
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

	// Create a claimed land owned by owner where visitors cannot trade/post.
	landID := w.newLandID(owner.ID)
	w.claims[landID] = &LandClaim{
		LandID:  landID,
		Owner:   owner.ID,
		Anchor:  owner.Pos,
		Radius:  32,
		Flags:   ClaimFlags{AllowBuild: false, AllowBreak: false, AllowDamage: false, AllowTrade: false},
		Members: map[string]bool{},
	}

	// Place a bulletin board block inside the claim.
	boardPos := owner.Pos
	bid := w.catalogs.Blocks.Index["BULLETIN_BOARD"]
	w.chunks.SetBlock(boardPos, bid)
	w.ensureBoard(boardPos)
	boardID := boardIDAt(boardPos)

	// Visitor is nearby but not a member: cannot post when allow_trade=false.
	visitor.Pos = boardPos
	w.applyInstant(visitor, protocol.InstantReq{
		ID:       "I_post_v",
		Type:     "POST_BOARD",
		TargetID: boardID,
		Title:    "hello",
		Body:     "world",
	}, 0)
	if len(visitor.Events) == 0 {
		t.Fatalf("expected action result event")
	}
	last := visitor.Events[len(visitor.Events)-1]
	if last["type"] != "ACTION_RESULT" || last["ref"] != "I_post_v" {
		t.Fatalf("unexpected last event: %+v", last)
	}
	if ok, _ := last["ok"].(bool); ok {
		t.Fatalf("expected visitor post to be rejected")
	}
	if code, _ := last["code"].(string); code != "E_NO_PERMISSION" {
		t.Fatalf("expected E_NO_PERMISSION, got %+v", last)
	}

	// Owner can post.
	w.applyInstant(owner, protocol.InstantReq{
		ID:       "I_post_o",
		Type:     "POST_BOARD",
		TargetID: boardID,
		Title:    "notice",
		Body:     "market day",
	}, 1)
	if len(w.boards[boardID].Posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(w.boards[boardID].Posts))
	}

	// OPEN should return board contents to anyone in range.
	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         visitor.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K_open", Type: "OPEN", TargetID: boardID},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: visitor.ID, Act: act}})

	found := false
	for _, ev := range visitor.Events {
		if ev["type"] == "BOARD" && ev["board_id"] == boardID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected BOARD event from OPEN")
	}

	// Too far: posting should fail.
	owner.Pos = Vec3i{X: boardPos.X + 10, Y: boardPos.Y, Z: boardPos.Z}
	owner.Events = nil
	w.applyInstant(owner, protocol.InstantReq{
		ID:       "I_post_far_o",
		Type:     "POST_BOARD",
		TargetID: boardID,
		Title:    "x",
		Body:     "y",
	}, 700)
	if len(owner.Events) == 0 {
		t.Fatalf("expected action result")
	}
	ev := owner.Events[len(owner.Events)-1]
	if ok, _ := ev["ok"].(bool); ok {
		t.Fatalf("expected posting to fail when too far")
	}
	if code, _ := ev["code"].(string); code != "E_BLOCKED" {
		t.Fatalf("expected E_BLOCKED for too far, got %+v", ev)
	}
}
