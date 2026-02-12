package worldtest

import (
	"fmt"
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestBlueprintAutoPull_PullsFromNearbyChest(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
	}, cats, "builder")

	self := h.LastObs().Self.Pos
	anchor := world.Vec3i{X: self[0], Y: 0, Z: self[2]}
	clearArea(t, h, anchor, 12)

	chestPos := world.Vec3i{X: anchor.X + 5, Y: 0, Z: anchor.Z}
	chestID := fmt.Sprintf("CHEST@%d,%d,%d", chestPos.X, chestPos.Y, chestPos.Z)

	// Create chest + stock it with planks via normal interactions (no world internals).
	h.AddInventory("CHEST", 1)
	h.AddInventory("PLANK", -1_000_000) // clear starter items
	h.AddInventory("PLANK", 10)

	h.Step(nil, []protocol.TaskReq{{
		ID:       "K_place_chest",
		Type:     "PLACE",
		ItemID:   "CHEST",
		BlockPos: chestPos.ToArray(),
	}}, nil)

	h.SetAgentPos(chestPos)
	h.Step(nil, []protocol.TaskReq{{
		ID:     "K_stock",
		Type:   "TRANSFER",
		Src:    "SELF",
		Dst:    chestID,
		ItemID: "PLANK",
		Count:  10,
	}}, nil)

	// Ensure the agent has no PLANK at build start.
	if got := invCount(h.LastObs().Inventory, "PLANK"); got != 0 {
		t.Fatalf("expected agent plank=0 before build, got %d", got)
	}

	h.ClearAgentEvents()
	h.SetAgentPos(anchor)

	// Build road_segment; it costs 5 PLANK and should auto-pull from the nearby chest.
	h.Step(nil, []protocol.TaskReq{{
		ID:          "K_build",
		Type:        "BUILD_BLUEPRINT",
		BlueprintID: "road_segment",
		Anchor:      anchor.ToArray(),
		Rotation:    0,
	}}, nil)
	// road_segment places 5 blocks at 2 blocks/tick => 3 ticks total including start tick.
	h.StepNoop()
	h.StepNoop()

	for _, task := range h.LastObs().Tasks {
		if task.Kind == "BUILD_BLUEPRINT" {
			t.Fatalf("expected build task done; tasks=%v", h.LastObs().Tasks)
		}
	}
	if got := invCount(h.LastObs().Inventory, "PLANK"); got != 0 {
		t.Fatalf("expected agent plank remaining=0, got %d", got)
	}

	// Verify chest inventory via OPEN projection.
	h.SetAgentPos(chestPos)
	obsOpen := h.Step(nil, []protocol.TaskReq{{ID: "K_open", Type: "OPEN", TargetID: chestID}}, nil)

	found := false
	plankCount := 0
	for _, e := range obsOpen.Events {
		if typ, _ := e["type"].(string); typ != "CONTAINER" {
			continue
		}
		if id, _ := e["container"].(string); id != chestID {
			continue
		}
		found = true
		if inv, ok := e["inventory"].([]interface{}); ok {
			for _, it := range inv {
				m, _ := it.(map[string]interface{})
				if m == nil {
					continue
				}
				item, _ := m["item"].(string)
				if item != "PLANK" {
					continue
				}
				if c, ok := m["count"].(float64); ok {
					plankCount = int(c)
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected CONTAINER event for chest open")
	}
	if plankCount != 5 {
		t.Fatalf("chest plank remaining: got %d want %d", plankCount, 5)
	}
}

func TestBlueprintAutoPull_SameLandOnly(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{
		ID:         "test",
		WorldType:  "OVERWORLD",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
	}, cats, "owner")

	claimAnchor := world.Vec3i{X: 0, Y: 0, Z: 0}
	clearArea(t, h, claimAnchor, 4)

	// Create a claim at origin. Claim cost is BATTERY + CRYSTAL_SHARD.
	h.AddInventory("BATTERY", 1)
	h.AddInventory("CRYSTAL_SHARD", 1)
	h.Step(nil, []protocol.TaskReq{{
		ID:     "K_claim",
		Type:   "CLAIM_LAND",
		Anchor: claimAnchor.ToArray(),
		Radius: 32,
	}}, nil)

	// Blueprint anchor is inside claim (edge), but a nearby chest is outside claim (still within pull range).
	anchor := world.Vec3i{X: 32, Y: 0, Z: 0}
	clearArea(t, h, anchor, 8)

	outsideChestPos := world.Vec3i{X: 64, Y: 0, Z: 0} // dist=32 from anchor, but outside claim.
	clearArea(t, h, outsideChestPos, 2)
	chestID := fmt.Sprintf("CHEST@%d,%d,%d", outsideChestPos.X, outsideChestPos.Y, outsideChestPos.Z)

	h.AddInventory("CHEST", 1)
	h.AddInventory("PLANK", -1_000_000) // clear starter items
	h.AddInventory("PLANK", 10)
	h.Step(nil, []protocol.TaskReq{{
		ID:       "K_place_chest",
		Type:     "PLACE",
		ItemID:   "CHEST",
		BlockPos: outsideChestPos.ToArray(),
	}}, nil)

	// Stock chest, then ensure self has 0 PLANK before build.
	h.SetAgentPos(outsideChestPos)
	h.Step(nil, []protocol.TaskReq{{
		ID:     "K_stock",
		Type:   "TRANSFER",
		Src:    "SELF",
		Dst:    chestID,
		ItemID: "PLANK",
		Count:  10,
	}}, nil)
	if got := invCount(h.LastObs().Inventory, "PLANK"); got != 0 {
		t.Fatalf("expected agent plank=0 before build, got %d", got)
	}

	h.ClearAgentEvents()

	h.Step(nil, []protocol.TaskReq{{
		ID:          "K_build",
		Type:        "BUILD_BLUEPRINT",
		BlueprintID: "road_segment",
		Anchor:      anchor.ToArray(),
		Rotation:    0,
	}}, nil)

	obs := h.LastObs()
	if !hasTaskFail(obs, "E_NO_RESOURCE") {
		t.Fatalf("expected TASK_FAIL E_NO_RESOURCE; events=%v", obs.Events)
	}
	if got := actionResultFieldString(obs, "K_build", "message"); got != "missing PLANK x5" {
		// The TASK_FAIL event carries message; ACTION_RESULT ref "K_build" is only for task start.
		// Assert by scanning TASK_FAIL message instead.
		msg := ""
		for _, e := range obs.Events {
			if e["type"] == "TASK_FAIL" {
				if s, _ := e["message"].(string); s != "" {
					msg = s
				}
			}
		}
		if msg != "missing PLANK x5" {
			t.Fatalf("unexpected failure message: %q", msg)
		}
	}
}
