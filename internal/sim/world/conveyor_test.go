package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestConveyor_MovesDroppedItemEntity(t *testing.T) {
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

	// Join an agent.
	respCh := make(chan JoinResponse, 1)
	w.step([]JoinRequest{{Name: "bot", Resp: respCh}}, nil, nil)
	jr := <-respCh
	a := w.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	// Place a conveyor facing +Z (yaw 0).
	a.Inventory["CONVEYOR"] = 1
	a.Yaw = 0
	pos := Vec3i{X: a.Pos.X, Y: 0, Z: a.Pos.Z}
	w.chunks.SetBlock(pos, w.chunks.gen.Air)
	w.chunks.SetBlock(Vec3i{X: pos.X, Y: 0, Z: pos.Z + 1}, w.chunks.gen.Air)

	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "PLACE", ItemID: "CONVEYOR", BlockPos: pos.ToArray()},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})

	if meta, ok := w.conveyors[pos]; !ok || !(meta.DX == 0 && meta.DZ == 1) {
		t.Fatalf("conveyor meta=%+v ok=%v want dx=0 dz=1", meta, ok)
	}

	// Spawn a dropped item entity on the conveyor and advance one tick.
	itemID := w.spawnItemEntity(w.CurrentTick(), "WORLD", pos, "IRON_INGOT", 2, "TEST")
	if itemID == "" {
		t.Fatalf("expected item entity id")
	}
	w.step(nil, nil, nil)

	e := w.items[itemID]
	if e == nil {
		t.Fatalf("missing item entity after move")
	}
	want := Vec3i{X: pos.X, Y: pos.Y, Z: pos.Z + 1}
	if e.Pos != want {
		t.Fatalf("item pos=%+v want %+v", e.Pos, want)
	}
}

func TestConveyor_InsertsIntoChest(t *testing.T) {
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

	respCh := make(chan JoinResponse, 1)
	w.step([]JoinRequest{{Name: "bot", Resp: respCh}}, nil, nil)
	jr := <-respCh
	a := w.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	a.Yaw = 0 // +Z
	pos := Vec3i{X: a.Pos.X, Y: 0, Z: a.Pos.Z}
	dst := Vec3i{X: pos.X, Y: pos.Y, Z: pos.Z + 1}
	w.chunks.SetBlock(pos, w.chunks.gen.Air)
	w.chunks.SetBlock(dst, w.chunks.gen.Air)

	// Place chest at destination.
	a.Inventory["CHEST"] = 1
	actChest := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "PLACE", ItemID: "CHEST", BlockPos: dst.ToArray()},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: actChest}})

	// Place conveyor at source.
	a.Inventory["CONVEYOR"] = 1
	actConv := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks: []protocol.TaskReq{
			{ID: "K2", Type: "PLACE", ItemID: "CONVEYOR", BlockPos: pos.ToArray()},
		},
	}
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: actConv}})

	itemID := w.spawnItemEntity(w.CurrentTick(), "WORLD", pos, "COAL", 3, "TEST")
	w.step(nil, nil, nil)

	if w.items[itemID] != nil {
		t.Fatalf("expected item entity to be inserted and despawned")
	}
	c := w.containers[dst]
	if c == nil {
		t.Fatalf("expected chest container runtime to exist")
	}
	if got := c.Inventory["COAL"]; got != 3 {
		t.Fatalf("chest coal=%d want 3", got)
	}
}

func TestConveyor_PullsFromBackChest_AndMovesToFrontChest(t *testing.T) {
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

	respCh := make(chan JoinResponse, 1)
	w.step([]JoinRequest{{Name: "bot", Resp: respCh}}, nil, nil)
	jr := <-respCh
	a := w.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	a.Yaw = 0 // +Z
	pos := Vec3i{X: a.Pos.X, Y: 0, Z: a.Pos.Z}
	back := Vec3i{X: pos.X, Y: 0, Z: pos.Z - 1}
	front := Vec3i{X: pos.X, Y: 0, Z: pos.Z + 1}
	w.chunks.SetBlock(pos, w.chunks.gen.Air)
	w.chunks.SetBlock(back, w.chunks.gen.Air)
	w.chunks.SetBlock(front, w.chunks.gen.Air)

	// Place back chest.
	a.Inventory["CHEST"] = 1
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "PLACE", ItemID: "CHEST", BlockPos: back.ToArray()}},
	}}})

	// Place front chest.
	a.Inventory["CHEST"] = 1
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K2", Type: "PLACE", ItemID: "CHEST", BlockPos: front.ToArray()}},
	}}})

	// Place conveyor between them.
	a.Inventory["CONVEYOR"] = 1
	w.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K3", Type: "PLACE", ItemID: "CONVEYOR", BlockPos: pos.ToArray()}},
	}}})

	// Put 2 coal in the back chest and let the belt move it forward.
	cb := w.containers[back]
	cf := w.containers[front]
	if cb == nil || cf == nil {
		t.Fatalf("expected chest containers to exist")
	}
	cb.Inventory["COAL"] = 2

	// Tick 1: pull 1 coal onto belt.
	w.step(nil, nil, nil)
	// Tick 2: insert 1 coal into front chest, pull the 2nd onto belt.
	w.step(nil, nil, nil)
	// Tick 3: insert 2nd coal.
	w.step(nil, nil, nil)

	if got := cf.Inventory["COAL"]; got != 2 {
		t.Fatalf("front chest coal=%d want 2", got)
	}
	if got := cb.Inventory["COAL"]; got != 0 {
		t.Fatalf("back chest coal=%d want 0", got)
	}
}
