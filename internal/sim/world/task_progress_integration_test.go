package world

import (
	"math"
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestTaskProgress_UsesRecipeAndBlueprintParams(t *testing.T) {
	t.Run("craft progress uses recipe time_ticks", func(t *testing.T) {
		cats, err := catalogs.Load("../../../configs")
		if err != nil {
			t.Fatalf("load catalogs: %v", err)
		}
		w, err := New(WorldConfig{ID: "test", Seed: 11}, cats)
		if err != nil {
			t.Fatalf("world: %v", err)
		}
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: "crafter", DeltaVoxels: false, Out: nil, Resp: resp})
		jr := <-resp
		a := w.agents[jr.Welcome.AgentID]
		if a == nil {
			t.Fatalf("missing agent")
		}
		w.tick.Store(1)

		a.Pos = Vec3i{X: 0, Y: 0, Z: 0}

		// Station nearby.
		benchID := w.catalogs.Blocks.Index["CRAFTING_BENCH"]
		benchPos := Vec3i{X: a.Pos.X + 1, Y: 0, Z: a.Pos.Z}
		w.chunks.SetBlock(benchPos, benchID)

		rec := w.catalogs.Recipes.ByID["chest"]
		if rec.RecipeID == "" || rec.TimeTicks != 5 {
			t.Fatalf("expected chest recipe time_ticks=5, got %+v", rec)
		}

		a.Inventory = map[string]int{"PLANK": 8}
		w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
			Type:            protocol.TypeAct,
			ProtocolVersion: protocol.Version,
			Tick:            w.CurrentTick(),
			AgentID:         a.ID,
			Tasks:           []protocol.TaskReq{{ID: "K1", Type: "CRAFT", RecipeID: "chest", Count: 1}},
		}}})

		// After 2 ticks of work: progress=2/5=0.4.
		w.step(nil, nil, nil)

		cl := &clientState{DeltaVoxels: false}
		obs := w.buildObs(a, cl, w.CurrentTick()-1)
		p := taskProgressByKind(obs.Tasks, "CRAFT")
		want := 2.0 / float64(rec.TimeTicks)
		if math.Abs(p-want) > 1e-6 {
			t.Fatalf("CRAFT progress=%v want %v", p, want)
		}
	})

	t.Run("smelt progress uses recipe time_ticks", func(t *testing.T) {
		cats, err := catalogs.Load("../../../configs")
		if err != nil {
			t.Fatalf("load catalogs: %v", err)
		}
		w, err := New(WorldConfig{ID: "test", Seed: 12}, cats)
		if err != nil {
			t.Fatalf("world: %v", err)
		}
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: "smelter", DeltaVoxels: false, Out: nil, Resp: resp})
		jr := <-resp
		a := w.agents[jr.Welcome.AgentID]
		if a == nil {
			t.Fatalf("missing agent")
		}
		w.tick.Store(1)

		a.Pos = Vec3i{X: 0, Y: 0, Z: 0}

		// Station nearby.
		furnaceID := w.catalogs.Blocks.Index["FURNACE"]
		furnacePos := Vec3i{X: a.Pos.X + 1, Y: 0, Z: a.Pos.Z}
		w.chunks.SetBlock(furnacePos, furnaceID)

		rec := w.smeltByInput["IRON_ORE"]
		if rec.RecipeID == "" || rec.TimeTicks != 10 {
			t.Fatalf("expected IRON_ORE smelt time_ticks=10, got %+v", rec)
		}

		a.Inventory = map[string]int{"IRON_ORE": 1, "COAL": 1}
		w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
			Type:            protocol.TypeAct,
			ProtocolVersion: protocol.Version,
			Tick:            w.CurrentTick(),
			AgentID:         a.ID,
			Tasks:           []protocol.TaskReq{{ID: "K1", Type: "SMELT", ItemID: "IRON_ORE", Count: 1}},
		}}})

		// After 3 ticks of work: progress=3/10=0.3.
		w.step(nil, nil, nil)
		w.step(nil, nil, nil)

		cl := &clientState{DeltaVoxels: false}
		obs := w.buildObs(a, cl, w.CurrentTick()-1)
		p := taskProgressByKind(obs.Tasks, "SMELT")
		want := 3.0 / float64(rec.TimeTicks)
		if math.Abs(p-want) > 1e-6 {
			t.Fatalf("SMELT progress=%v want %v", p, want)
		}
	})

	t.Run("blueprint progress uses build index", func(t *testing.T) {
		cats, err := catalogs.Load("../../../configs")
		if err != nil {
			t.Fatalf("load catalogs: %v", err)
		}
		w, err := New(WorldConfig{ID: "test", Seed: 13, BlueprintBlocksPerTick: 1}, cats)
		if err != nil {
			t.Fatalf("world: %v", err)
		}
		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: "builder", DeltaVoxels: false, Out: nil, Resp: resp})
		jr := <-resp
		a := w.agents[jr.Welcome.AgentID]
		if a == nil {
			t.Fatalf("missing agent")
		}
		w.tick.Store(1)

		a.Pos = Vec3i{X: 0, Y: 0, Z: 0}
		a.Inventory = map[string]int{"PLANK": 5}

		bp := w.catalogs.Blueprints.ByID["road_segment"]
		if bp.ID == "" || len(bp.Blocks) != 5 {
			t.Fatalf("unexpected road_segment blueprint: id=%q blocks=%d", bp.ID, len(bp.Blocks))
		}

		anchor := Vec3i{X: a.Pos.X + 4, Y: 0, Z: a.Pos.Z}
		// Ensure blueprint footprint is empty.
		for i := 0; i < len(bp.Blocks); i++ {
			w.chunks.SetBlock(Vec3i{X: anchor.X, Y: 0, Z: anchor.Z + i}, w.chunks.gen.Air)
		}
		w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
			Type:            protocol.TypeAct,
			ProtocolVersion: protocol.Version,
			Tick:            w.CurrentTick(),
			AgentID:         a.ID,
			Tasks:           []protocol.TaskReq{{ID: "K1", Type: "BUILD_BLUEPRINT", BlueprintID: "road_segment", Anchor: anchor.ToArray(), Rotation: 0}},
		}}})

		cl := &clientState{DeltaVoxels: false}
		obs := w.buildObs(a, cl, w.CurrentTick()-1)
		p := taskProgressByKind(obs.Tasks, "BUILD_BLUEPRINT")
		if p <= 0 || p > 1 {
			t.Fatalf("BUILD_BLUEPRINT progress=%v want in (0,1]", p)
		}
		want := 1.0 / float64(len(bp.Blocks))
		if math.Abs(p-want) > 1e-6 {
			t.Fatalf("BUILD_BLUEPRINT progress=%v want %v", p, want)
		}
	})
}

func taskProgressByKind(tasksObs []protocol.TaskObs, kind string) float64 {
	for _, t := range tasksObs {
		if t.Kind == kind {
			return t.Progress
		}
	}
	return 0
}
