package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestStartEvent_CrystalRift_SpawnsOreAndBroadcastsLocation(t *testing.T) {
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

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "a", DeltaVoxels: false, Out: nil, Resp: resp})
	j := <-resp
	a := w.agents[j.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}
	a.Events = nil

	w.startEvent(0, "CRYSTAL_RIFT")

	if w.activeEventID != "CRYSTAL_RIFT" {
		t.Fatalf("activeEventID=%q", w.activeEventID)
	}
	if w.activeEventStart != 0 {
		t.Fatalf("activeEventStart=%d want 0", w.activeEventStart)
	}
	if w.activeEventEnds == 0 {
		t.Fatalf("expected non-zero activeEventEnds")
	}
	if w.activeEventRadius <= 0 {
		t.Fatalf("expected activeEventRadius > 0")
	}
	if w.activeEventCenter == (Vec3i{}) {
		t.Fatalf("expected non-zero activeEventCenter")
	}

	found := false
	for _, e := range a.Events {
		if e["type"] == "WORLD_EVENT" && e["event_id"] == "CRYSTAL_RIFT" {
			found = true
			if e["radius"] == nil || e["center"] == nil {
				t.Fatalf("expected WORLD_EVENT to include center+radius, got=%v", e)
			}
		}
	}
	if !found {
		t.Fatalf("expected WORLD_EVENT broadcast")
	}

	oreID := cats.Blocks.Index["CRYSTAL_ORE"]
	// 2D world: deterministic 5x5 surface cluster centered at (center.X, y=0, center.Z).
	for dz := -2; dz <= 2; dz++ {
		for dx := -2; dx <= 2; dx++ {
			p := Vec3i{X: w.activeEventCenter.X + dx, Y: 0, Z: w.activeEventCenter.Z + dz}
			if got := w.chunks.GetBlock(p); got != oreID {
				t.Fatalf("ore cluster mismatch at %+v: got %d want %d", p, got, oreID)
			}
		}
	}
}

func TestCrystalRift_MiningInZoneGivesBonusShard(t *testing.T) {
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

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "miner", DeltaVoxels: false, Out: nil, Resp: resp})
	j := <-resp
	a := w.agents[j.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	w.startEvent(0, "CRYSTAL_RIFT")

	// Mine the center ore block.
	target := Vec3i{X: w.activeEventCenter.X, Y: 0, Z: w.activeEventCenter.Z}
	a.Pos = target

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         a.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "MINE", BlockPos: target.ToArray()},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})
	// WorkTicks reaches 10 on tick 9 (inclusive).
	for i := 0; i < 9; i++ {
		w.step(nil, nil, nil)
	}

	// Event bonus goes directly to inventory; base drop is an ITEM entity.
	if got := a.Inventory["CRYSTAL_SHARD"]; got != 1 {
		t.Fatalf("CRYSTAL_SHARD=%d want 1 (event bonus only)", got)
	}
	if ids := w.itemsAt[target]; len(ids) == 0 {
		t.Fatalf("expected item entity drop at mined pos")
	} else {
		id := ids[0]
		e := w.items[id]
		if e == nil || e.Item != "CRYSTAL_SHARD" || e.Count != 1 {
			t.Fatalf("unexpected drop entity: %+v", e)
		}
		// Pick up the drop to verify full reward path.
		act2 := protocol.ActMsg{
			Type:            protocol.TypeAct,
			ProtocolVersion: protocol.Version,
			Tick:            w.CurrentTick(),
			AgentID:         a.ID,
			Tasks:           []protocol.TaskReq{{ID: "K2", Type: "GATHER", TargetID: id}},
		}
		w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act2}})
		if got := a.Inventory["CRYSTAL_SHARD"]; got != 2 {
			t.Fatalf("CRYSTAL_SHARD=%d want 2 (base drop + event bonus)", got)
		}
	}

	foundGoal := false
	for _, e := range a.Events {
		if e["type"] == "EVENT_GOAL" && e["event_id"] == "CRYSTAL_RIFT" {
			foundGoal = true
			break
		}
	}
	if !foundGoal {
		t.Fatalf("expected EVENT_GOAL from mining in event zone")
	}
}

func TestStartEvent_RuinsGate_SpawnsLootChestAndOpenAwardsGoal(t *testing.T) {
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

	resp := make(chan JoinResponse, 1)
	w.handleJoin(JoinRequest{Name: "explorer", DeltaVoxels: false, Out: nil, Resp: resp})
	j := <-resp
	a := w.agents[j.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}
	a.Events = nil

	w.startEvent(0, "RUINS_GATE")

	if w.activeEventID != "RUINS_GATE" {
		t.Fatalf("activeEventID=%q", w.activeEventID)
	}
	if w.activeEventRadius <= 0 || w.activeEventCenter == (Vec3i{}) {
		t.Fatalf("expected event center+radius")
	}

	chestID := cats.Blocks.Index["CHEST"]
	if got := w.chunks.GetBlock(w.activeEventCenter); got != chestID {
		t.Fatalf("expected chest at center: got %d want %d", got, chestID)
	}
	c := w.containers[w.activeEventCenter]
	if c == nil || c.Type != "CHEST" {
		t.Fatalf("expected CHEST container at center")
	}
	if c.Inventory["CRYSTAL_SHARD"] <= 0 {
		t.Fatalf("expected loot in chest")
	}

	// OPEN it (teleport within range).
	a.Pos = w.activeEventCenter
	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            0,
		AgentID:         a.ID,
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "OPEN", TargetID: c.ID()},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})

	foundGoal := false
	for _, e := range a.Events {
		if e["type"] == "EVENT_GOAL" && e["event_id"] == "RUINS_GATE" {
			foundGoal = true
			break
		}
	}
	if !foundGoal {
		t.Fatalf("expected EVENT_GOAL from opening ruins chest")
	}
}
