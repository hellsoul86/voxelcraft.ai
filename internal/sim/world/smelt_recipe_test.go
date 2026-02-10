package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestSmelt_ConfigDrivenRecipes(t *testing.T) {
	setup := func(t *testing.T) (*World, *Agent) {
		t.Helper()
		cats, err := catalogs.Load("../../../configs")
		if err != nil {
			t.Fatalf("load catalogs: %v", err)
		}
		w, err := New(WorldConfig{ID: "test", Seed: 9}, cats)
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
		// Avoid tick-0 scripted events.
		w.tick.Store(1)

		// Ensure furnace nearby.
		furnace := w.catalogs.Blocks.Index["FURNACE"]
		fp := Vec3i{X: a.Pos.X + 1, Y: a.Pos.Y, Z: a.Pos.Z}
		w.chunks.SetBlock(fp, furnace)

		a.Inventory = map[string]int{}
		return w, a
	}

	t.Run("unsupported item rejected at task start", func(t *testing.T) {
		w, a := setup(t)

		act := protocol.ActMsg{
			Type:            protocol.TypeAct,
			ProtocolVersion: protocol.Version,
			Tick:            w.CurrentTick(),
			AgentID:         a.ID,
			Tasks:           []protocol.TaskReq{{ID: "K1", Type: "SMELT", ItemID: "PLANK", Count: 1}},
		}
		w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})

		if a.WorkTask != nil {
			t.Fatalf("expected no work task for unsupported smelt item")
		}

		found := false
		for _, e := range a.Events {
			if e["type"] != "ACTION_RESULT" || e["ref"] != "K1" {
				continue
			}
			found = true
			if ok, _ := e["ok"].(bool); ok {
				t.Fatalf("expected ok=false for unsupported smelt")
			}
			if code, _ := e["code"].(string); code != "E_INVALID_TARGET" {
				t.Fatalf("expected E_INVALID_TARGET, got %v", e["code"])
			}
			break
		}
		if !found {
			t.Fatalf("expected ACTION_RESULT for rejected SMELT task")
		}
	})

	t.Run("iron ore smelt uses recipe time/inputs/outputs", func(t *testing.T) {
		w, a := setup(t)
		rec := w.smeltByInput["IRON_ORE"]
		if rec.RecipeID == "" || rec.TimeTicks <= 0 {
			t.Fatalf("missing smelt recipe for IRON_ORE")
		}

		a.Inventory["IRON_ORE"] = 1
		a.Inventory["COAL"] = 1

		act := protocol.ActMsg{
			Type:            protocol.TypeAct,
			ProtocolVersion: protocol.Version,
			Tick:            w.CurrentTick(),
			AgentID:         a.ID,
			Tasks:           []protocol.TaskReq{{ID: "K1", Type: "SMELT", ItemID: "IRON_ORE", Count: 1}},
		}
		w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})

		// Not done before time_ticks.
		for i := 0; i < rec.TimeTicks-2; i++ {
			w.step(nil, nil, nil)
		}
		if got := a.Inventory["IRON_INGOT"]; got != 0 {
			t.Fatalf("smelt too early: IRON_INGOT=%d want 0", got)
		}
		if got := a.Inventory["IRON_ORE"]; got != 1 {
			t.Fatalf("inputs consumed too early: IRON_ORE=%d want 1", got)
		}

		// Completion tick.
		w.step(nil, nil, nil)
		if got := a.Inventory["IRON_ORE"]; got != 0 {
			t.Fatalf("IRON_ORE=%d want 0", got)
		}
		if got := a.Inventory["COAL"]; got != 0 {
			t.Fatalf("COAL=%d want 0", got)
		}
		if got := a.Inventory["IRON_INGOT"]; got != 1 {
			t.Fatalf("IRON_INGOT=%d want 1", got)
		}
	})

	t.Run("raw meat smelt to cooked meat", func(t *testing.T) {
		w, a := setup(t)
		rec := w.smeltByInput["RAW_MEAT"]
		if rec.RecipeID == "" || rec.TimeTicks <= 0 {
			t.Fatalf("missing smelt recipe for RAW_MEAT")
		}
		a.Inventory["RAW_MEAT"] = 1
		a.Inventory["COAL"] = 1

		act := protocol.ActMsg{
			Type:            protocol.TypeAct,
			ProtocolVersion: protocol.Version,
			Tick:            w.CurrentTick(),
			AgentID:         a.ID,
			Tasks:           []protocol.TaskReq{{ID: "K1", Type: "SMELT", ItemID: "RAW_MEAT", Count: 1}},
		}
		w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})

		for i := 0; i < rec.TimeTicks-2; i++ {
			w.step(nil, nil, nil)
		}
		if got := a.Inventory["COOKED_MEAT"]; got != 0 {
			t.Fatalf("smelt too early: COOKED_MEAT=%d want 0", got)
		}

		w.step(nil, nil, nil)
		if got := a.Inventory["RAW_MEAT"]; got != 0 {
			t.Fatalf("RAW_MEAT=%d want 0", got)
		}
		if got := a.Inventory["COAL"]; got != 0 {
			t.Fatalf("COAL=%d want 0", got)
		}
		if got := a.Inventory["COOKED_MEAT"]; got != 1 {
			t.Fatalf("COOKED_MEAT=%d want 1", got)
		}
	})
}
