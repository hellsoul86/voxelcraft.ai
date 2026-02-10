package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestMining_ImplicitToolsAffectSpeedAndStamina(t *testing.T) {
	setup := func(t *testing.T) (*World, *Agent, Vec3i, uint16) {
		t.Helper()
		cats, err := catalogs.Load("../../../configs")
		if err != nil {
			t.Fatalf("load catalogs: %v", err)
		}
		w, err := New(WorldConfig{ID: "test", Seed: 7}, cats)
		if err != nil {
			t.Fatalf("world: %v", err)
		}

		resp := make(chan JoinResponse, 1)
		w.handleJoin(JoinRequest{Name: "miner", DeltaVoxels: false, Out: nil, Resp: resp})
		jr := <-resp
		a := w.agents[jr.Welcome.AgentID]
		if a == nil {
			t.Fatalf("missing agent")
		}

		// Start at tick 1 to avoid day-0 director scripting and hunger tick-down at tick 0.
		w.tick.Store(1)

		// Deterministic stamina accounting: disable stamina recovery.
		a.Hunger = 0
		a.StaminaMilli = 1000
		a.Inventory = map[string]int{}

		pos := Vec3i{X: a.Pos.X + 1, Y: a.Pos.Y, Z: a.Pos.Z}
		stone := w.catalogs.Blocks.Index["STONE"]
		w.chunks.SetBlock(pos, stone)
		return w, a, pos, stone
	}

	t.Run("no tool", func(t *testing.T) {
		w, a, pos, stone := setup(t)

		act := protocol.ActMsg{
			Type:            protocol.TypeAct,
			ProtocolVersion: protocol.Version,
			Tick:            w.CurrentTick(),
			AgentID:         a.ID,
			Tasks:           []protocol.TaskReq{{ID: "K1", Type: "MINE", BlockPos: pos.ToArray()}},
		}
		w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})

		// After 9 total mine ticks, the block should still be present.
		for i := 0; i < 8; i++ {
			w.step(nil, nil, nil)
		}
		if got := w.chunks.GetBlock(pos); got != stone {
			t.Fatalf("stone mined too early: got %d want %d", got, stone)
		}

		// 10th mine tick completes the break.
		w.step(nil, nil, nil)
		if got := w.chunks.GetBlock(pos); got == stone {
			t.Fatalf("expected stone mined on 10th tick")
		}
		if got := a.Inventory["STONE"]; got != 1 {
			t.Fatalf("drop: STONE=%d want 1", got)
		}
		if got, want := a.StaminaMilli, 1000-10*15; got != want {
			t.Fatalf("stamina: got %d want %d", got, want)
		}
	})

	t.Run("iron pickaxe", func(t *testing.T) {
		w, a, pos, stone := setup(t)
		a.Inventory["IRON_PICKAXE"] = 1

		act := protocol.ActMsg{
			Type:            protocol.TypeAct,
			ProtocolVersion: protocol.Version,
			Tick:            w.CurrentTick(),
			AgentID:         a.ID,
			Tasks:           []protocol.TaskReq{{ID: "K1", Type: "MINE", BlockPos: pos.ToArray()}},
		}
		w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})

		// After 3 total mine ticks, it should not be done.
		for i := 0; i < 2; i++ {
			w.step(nil, nil, nil)
		}
		if got := w.chunks.GetBlock(pos); got != stone {
			t.Fatalf("stone mined too early with tool: got %d want %d", got, stone)
		}

		// 4th mine tick completes the break.
		w.step(nil, nil, nil)
		if got := w.chunks.GetBlock(pos); got == stone {
			t.Fatalf("expected stone mined on 4th tick with iron pickaxe")
		}
		if got := a.Inventory["STONE"]; got != 1 {
			t.Fatalf("drop: STONE=%d want 1", got)
		}
		if got, want := a.StaminaMilli, 1000-4*9; got != want {
			t.Fatalf("stamina: got %d want %d", got, want)
		}
	})
}

