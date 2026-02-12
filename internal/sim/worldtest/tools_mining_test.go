package worldtest

import (
	"math"
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestMining_ImplicitToolsAffectSpeedAndStamina(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	t.Run("no tool", func(t *testing.T) {
		h := NewHarness(t, world.WorldConfig{
			ID:           "test",
			Seed:         7,
			StarterItems: map[string]int{},
		}, cats, "miner")
		h.StepNoop() // tick 1

		if ok := h.W.DebugSetAgentVitals(h.DefaultAgentID, -1, 0, 1000); !ok {
			t.Fatalf("DebugSetAgentVitals returned false")
		}

		self := h.LastObs().Self.Pos
		pos := world.Vec3i{X: self[0] + 1, Y: 0, Z: self[2]}
		if err := h.W.DebugSetBlock(pos, "STONE"); err != nil {
			t.Fatalf("DebugSetBlock(STONE): %v", err)
		}

		h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "MINE", BlockPos: pos.ToArray()}}, nil)

		// After 9 total mine ticks, the block should still be present.
		for i := 0; i < 8; i++ {
			h.StepNoop()
		}
		got, err := h.W.DebugGetBlock(pos)
		if err != nil {
			t.Fatalf("DebugGetBlock: %v", err)
		}
		stoneID := cats.Blocks.Index["STONE"]
		if got != stoneID {
			t.Fatalf("stone mined too early: got %d want %d", got, stoneID)
		}

		// 10th mine tick completes the break.
		h.StepNoop()
		got, err = h.W.DebugGetBlock(pos)
		if err != nil {
			t.Fatalf("DebugGetBlock: %v", err)
		}
		if got == stoneID {
			t.Fatalf("expected stone mined on 10th tick")
		}

		// Drop should be an ITEM entity at the mined position.
		foundDrop := false
		obs := h.LastObs()
		for _, e := range obs.Entities {
			if e.Type != "ITEM" {
				continue
			}
			if e.Pos == pos.ToArray() && e.Item == "STONE" && e.Count == 1 {
				foundDrop = true
				break
			}
		}
		if !foundDrop {
			t.Fatalf("expected ITEM drop at %+v; entities=%v", pos, obs.Entities)
		}

		// Stamina should match exact mining cost for the duration (hunger=0 disables recovery).
		want := float64(1000-10*15) / 1000.0
		if got := obs.Self.Stamina; math.Abs(got-want) > 1e-9 {
			t.Fatalf("stamina: got %.6f want %.6f", got, want)
		}
	})

	t.Run("iron pickaxe", func(t *testing.T) {
		h := NewHarness(t, world.WorldConfig{
			ID:           "test",
			Seed:         7,
			StarterItems: map[string]int{"IRON_PICKAXE": 1},
		}, cats, "miner")
		h.StepNoop()

		if ok := h.W.DebugSetAgentVitals(h.DefaultAgentID, -1, 0, 1000); !ok {
			t.Fatalf("DebugSetAgentVitals returned false")
		}

		self := h.LastObs().Self.Pos
		pos := world.Vec3i{X: self[0] + 1, Y: 0, Z: self[2]}
		if err := h.W.DebugSetBlock(pos, "STONE"); err != nil {
			t.Fatalf("DebugSetBlock(STONE): %v", err)
		}

		h.Step(nil, []protocol.TaskReq{{ID: "K1", Type: "MINE", BlockPos: pos.ToArray()}}, nil)

		// After 3 total mine ticks, it should not be done.
		h.StepNoop()
		h.StepNoop()
		got, err := h.W.DebugGetBlock(pos)
		if err != nil {
			t.Fatalf("DebugGetBlock: %v", err)
		}
		stoneID := cats.Blocks.Index["STONE"]
		if got != stoneID {
			t.Fatalf("stone mined too early with tool: got %d want %d", got, stoneID)
		}

		// 4th mine tick completes the break.
		h.StepNoop()
		got, err = h.W.DebugGetBlock(pos)
		if err != nil {
			t.Fatalf("DebugGetBlock: %v", err)
		}
		if got == stoneID {
			t.Fatalf("expected stone mined on 4th tick with iron pickaxe")
		}

		// Drop should be an ITEM entity at the mined position.
		foundDrop := false
		obs := h.LastObs()
		for _, e := range obs.Entities {
			if e.Type != "ITEM" {
				continue
			}
			if e.Pos == pos.ToArray() && e.Item == "STONE" && e.Count == 1 {
				foundDrop = true
				break
			}
		}
		if !foundDrop {
			t.Fatalf("expected ITEM drop at %+v; entities=%v", pos, obs.Entities)
		}

		want := float64(1000-4*9) / 1000.0
		if got := obs.Self.Stamina; math.Abs(got-want) > 1e-9 {
			t.Fatalf("stamina: got %.6f want %.6f", got, want)
		}
	})
}
