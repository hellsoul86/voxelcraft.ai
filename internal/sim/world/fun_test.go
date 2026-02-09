package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestFun_NoveltyBiomeOnJoin(t *testing.T) {
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
	w.handleJoin(JoinRequest{Name: "bot", DeltaVoxels: false, Out: nil, Resp: resp})
	r := <-resp
	a := w.agents[r.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}
	if got := a.Fun.Novelty; got != 10 {
		t.Fatalf("novelty on join: got %d want %d", got, 10)
	}
}

func TestFun_NoveltyRecipeOnFirstCraft(t *testing.T) {
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
	w.handleJoin(JoinRequest{Name: "bot", DeltaVoxels: false, Out: nil, Resp: resp})
	r := <-resp
	a := w.agents[r.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	// CRAFT stick_from_plank (HAND, tier 1 => +3 novelty).
	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         a.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "CRAFT", RecipeID: "stick_from_plank", Count: 1},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})
	// time_ticks=5, so advance to completion.
	for i := 0; i < 5; i++ {
		w.step(nil, nil, nil)
	}

	if got, want := a.Fun.Novelty, 13; got != want {
		t.Fatalf("novelty after first craft: got %d want %d", got, want)
	}
}

func TestFun_CreationDelayedAwardOnBlueprintSurvival(t *testing.T) {
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
	w.handleJoin(JoinRequest{Name: "bot", DeltaVoxels: false, Out: nil, Resp: resp})
	r := <-resp
	a := w.agents[r.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	anchor := Vec3i{X: a.Pos.X, Y: 40, Z: a.Pos.Z}
	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         a.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "BUILD_BLUEPRINT", BlueprintID: "road_segment", Anchor: anchor.ToArray(), Rotation: 0},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})
	w.step(nil, nil, nil)
	w.step(nil, nil, nil) // should complete

	if got := a.Fun.Creation; got != 0 {
		t.Fatalf("creation should be delayed until survival: got %d", got)
	}
	if len(w.structures) != 1 {
		t.Fatalf("expected 1 structure, got %d", len(w.structures))
	}
	var s *Structure
	for _, v := range w.structures {
		s = v
	}
	if s == nil {
		t.Fatalf("missing structure")
	}

	// Fast-forward due tick in test to avoid 3000 ticks.
	s.AwardDueTick = w.CurrentTick()
	w.step(nil, nil, nil)

	if got := a.Fun.Creation; got <= 0 {
		t.Fatalf("creation should be awarded after due tick: got %d", got)
	}
}
