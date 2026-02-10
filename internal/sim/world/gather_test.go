package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestTask_Gather_PicksUpItemEntityAndRemovesIt(t *testing.T) {
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
	w.handleJoin(JoinRequest{Name: "picker", DeltaVoxels: false, Out: nil, Resp: resp})
	j := <-resp
	a := w.agents[j.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	dropPos := Vec3i{X: a.Pos.X + 1, Y: a.Pos.Y, Z: a.Pos.Z}
	id := w.spawnItemEntity(0, "WORLD", dropPos, "PLANK", 5, "TEST")
	if id == "" {
		t.Fatalf("spawnItemEntity returned empty id")
	}

	// Item entities should be visible via OBS.entities when in range.
	obs := w.buildObs(a, &clientState{DeltaVoxels: false}, 0)
	found := false
	for _, e := range obs.Entities {
		if e.Type == "ITEM" && e.ID == id && e.Pos == dropPos.ToArray() && e.Item == "PLANK" && e.Count == 5 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected ITEM entity %s in OBS", id)
	}

	start := a.Inventory["PLANK"]
	a.Pos = dropPos

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         a.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "GATHER", TargetID: id},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})

	if got := a.Inventory["PLANK"]; got != start+5 {
		t.Fatalf("inventory after gather: PLANK=%d want %d", got, start+5)
	}
	if w.items[id] != nil {
		t.Fatalf("expected item entity to be removed after gather")
	}
}
