package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestTask_Gather_PicksUpItemEntityAndRemovesIt(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{ID: "test", Seed: 42}, cats, "picker")

	self := h.LastObs().Self.Pos
	minePos := world.Vec3i{X: self[0] + 1, Y: 0, Z: self[2]}
	h.SetBlock(minePos, "CRYSTAL_ORE")
	h.AddInventory("IRON_PICKAXE", 1)

	h.Step(nil, []protocol.TaskReq{{
		ID:       "K_mine",
		Type:     "MINE",
		BlockPos: minePos.ToArray(),
	}}, nil)
	obs := stepUntilWorkDone(t, h, "MINE", 50)

	itemID, item, count := findItemEntityAt(obs, minePos.ToArray())
	if itemID == "" || item == "" || count <= 0 {
		t.Fatalf("expected ITEM entity after MINE at %v; entities=%v", minePos, obs.Entities)
	}

	start := invCount(obs.Inventory, item)
	h.SetAgentPos(minePos)
	h.ClearAgentEvents()
	obs = h.Step(nil, []protocol.TaskReq{{
		ID:       "K_gather",
		Type:     "GATHER",
		TargetID: itemID,
	}}, nil)
	if hasTaskFail(obs, "") {
		t.Fatalf("expected GATHER to succeed; events=%v", obs.Events)
	}
	if got := invCount(obs.Inventory, item); got != start+count {
		t.Fatalf("inventory after gather: %s=%d want %d", item, got, start+count)
	}
	for _, e := range obs.Entities {
		if e.ID == itemID {
			t.Fatalf("expected item entity %s to be removed after gather; entities=%v", itemID, obs.Entities)
		}
	}
}

func TestTask_Gather_DeniedForVisitorsInClaim(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	h := NewHarness(t, world.WorldConfig{ID: "test", WorldType: "OVERWORLD", Seed: 31}, cats, "owner")
	owner := h.DefaultAgentID
	visitor := h.Join("visitor")

	anchorArr := h.LastObsFor(owner).Self.Pos
	anchor := world.Vec3i{X: anchorArr[0], Y: 0, Z: anchorArr[2]}
	h.SetBlock(anchor, "AIR")
	h.AddInventoryFor(owner, "BATTERY", 1)
	h.AddInventoryFor(owner, "CRYSTAL_SHARD", 1)

	h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:     "K_claim",
		Type:   "CLAIM_LAND",
		Anchor: anchorArr,
		Radius: 8,
	}}, nil)
	if got := actionResultCode(h.LastObsFor(owner), "K_claim"); got != "" {
		t.Fatalf("claim expected ok, got code=%q events=%v", got, h.LastObsFor(owner).Events)
	}

	minePos := world.Vec3i{X: anchor.X + 1, Y: 0, Z: anchor.Z}
	h.SetBlock(minePos, "CRYSTAL_ORE")
	h.AddInventoryFor(owner, "IRON_PICKAXE", 1)

	h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:       "K_mine",
		Type:     "MINE",
		BlockPos: minePos.ToArray(),
	}}, nil)
	obsOwner := stepUntilWorkDoneFor(t, h, owner, "MINE", 50)

	itemID, item, count := findItemEntityAt(obsOwner, minePos.ToArray())
	if itemID == "" || item == "" || count <= 0 {
		t.Fatalf("expected ITEM entity after MINE at %v; entities=%v", minePos, obsOwner.Entities)
	}

	// Visitor cannot pick up items in a protected claim.
	h.SetAgentPosFor(visitor, minePos)
	h.ClearAgentEventsFor(visitor)
	obsV := h.StepFor(visitor, nil, []protocol.TaskReq{{
		ID:       "K_gather",
		Type:     "GATHER",
		TargetID: itemID,
	}}, nil)
	if !hasTaskFail(obsV, "E_NO_PERMISSION") {
		t.Fatalf("expected E_NO_PERMISSION for visitor pickup; events=%v", obsV.Events)
	}
	if got := invCount(obsV.Inventory, item); got != 0 {
		t.Fatalf("visitor inventory changed on denied pickup: %s=%d", item, got)
	}
	// Item should remain.
	if id2, _, _ := findItemEntityAt(obsV, minePos.ToArray()); id2 == "" {
		t.Fatalf("expected item entity to remain after denied pickup; entities=%v", obsV.Entities)
	}

	// Owner can pick it up.
	h.SetAgentPosFor(owner, minePos)
	h.ClearAgentEventsFor(owner)
	obsOwner2 := h.StepFor(owner, nil, []protocol.TaskReq{{
		ID:       "K_gather2",
		Type:     "GATHER",
		TargetID: itemID,
	}}, nil)
	if hasTaskFail(obsOwner2, "") {
		t.Fatalf("expected owner gather to succeed; events=%v", obsOwner2.Events)
	}
	if got := invCount(obsOwner2.Inventory, item); got < count {
		t.Fatalf("owner inventory after gather: %s=%d want >=%d", item, got, count)
	}
	for _, e := range obsOwner2.Entities {
		if e.ID == itemID {
			t.Fatalf("expected item entity removed after owner pickup; entities=%v", obsOwner2.Entities)
		}
	}
}

func findItemEntityAt(obs protocol.ObsMsg, pos [3]int) (id, item string, count int) {
	for _, e := range obs.Entities {
		if e.Type == "ITEM" && e.Pos == pos {
			return e.ID, e.Item, e.Count
		}
	}
	return "", "", 0
}

func stepUntilWorkDone(t *testing.T, h *Harness, kind string, max int) protocol.ObsMsg {
	return stepUntilWorkDoneFor(t, h, h.DefaultAgentID, kind, max)
}

func stepUntilWorkDoneFor(t *testing.T, h *Harness, agentID string, kind string, max int) protocol.ObsMsg {
	t.Helper()
	for i := 0; i < max; i++ {
		obs := h.LastObsFor(agentID)
		for _, e := range obs.Events {
			if e["type"] == "TASK_DONE" && e["kind"] == kind {
				return obs
			}
		}
		if hasTaskFail(obs, "") {
			t.Fatalf("unexpected TASK_FAIL while waiting for %s; events=%v", kind, obs.Events)
		}
		h.StepNoop()
	}
	t.Fatalf("timeout waiting for %s completion; last=%v", kind, h.LastObsFor(agentID).Tasks)
	return protocol.ObsMsg{}
}
