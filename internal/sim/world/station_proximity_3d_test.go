package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestStationProximity2D_CraftBenchAdjacent(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{ID: "test", Seed: 1}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "crafter", DeltaVoxels: false, Out: make(chan []byte, 1), Resp: resp})
	jr := <-resp
	a := w.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}
	// Avoid tick-0 scripted events.
	w.tick.Store(1)

	a.Pos = Vec3i{X: a.Pos.X, Y: 0, Z: a.Pos.Z}

	// Place a bench next to agent.
	benchID := w.catalogs.Blocks.Index["CRAFTING_BENCH"]
	benchPos := Vec3i{X: a.Pos.X + 1, Y: 0, Z: a.Pos.Z}
	w.chunks.SetBlock(benchPos, benchID)

	rec := w.catalogs.Recipes.ByID["chest"]
	if rec.RecipeID == "" || rec.TimeTicks <= 0 {
		t.Fatalf("missing chest recipe")
	}

	a.Inventory = map[string]int{"PLANK": 8}

	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         a.ID,
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "CRAFT", RecipeID: "chest", Count: 1}},
	}}})

	for i := 0; i < rec.TimeTicks+2; i++ {
		w.step(nil, nil, nil)
	}

	if got := a.Inventory["PLANK"]; got != 0 {
		t.Fatalf("plank not consumed: got %d want 0", got)
	}
	if got := a.Inventory["CHEST"]; got != 1 {
		t.Fatalf("chest not crafted: got %d want 1", got)
	}
}

func TestStationProximity2D_SmeltFurnaceAdjacent(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{ID: "test", Seed: 2}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "smelter", DeltaVoxels: false, Out: make(chan []byte, 1), Resp: resp})
	jr := <-resp
	a := w.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}
	// Avoid tick-0 scripted events.
	w.tick.Store(1)

	a.Pos = Vec3i{X: a.Pos.X, Y: 0, Z: a.Pos.Z}

	// Place a furnace next to agent.
	furnaceID := w.catalogs.Blocks.Index["FURNACE"]
	furnacePos := Vec3i{X: a.Pos.X + 1, Y: 0, Z: a.Pos.Z}
	w.chunks.SetBlock(furnacePos, furnaceID)

	rec := w.smeltByInput["IRON_ORE"]
	if rec.RecipeID == "" || rec.TimeTicks <= 0 {
		t.Fatalf("missing smelt recipe for IRON_ORE")
	}

	a.Inventory = map[string]int{"IRON_ORE": 1, "COAL": 1}

	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		AgentID:         a.ID,
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "SMELT", ItemID: "IRON_ORE", Count: 1}},
	}}})

	for i := 0; i < rec.TimeTicks+2; i++ {
		w.step(nil, nil, nil)
	}

	if got := a.Inventory["IRON_ORE"]; got != 0 {
		t.Fatalf("IRON_ORE=%d want 0", got)
	}
	if got := a.Inventory["COAL"]; got != 0 {
		t.Fatalf("COAL=%d want 0", got)
	}
	if got := a.Inventory["IRON_INGOT"]; got != 1 {
		t.Fatalf("IRON_INGOT=%d want 1", got)
	}
}
