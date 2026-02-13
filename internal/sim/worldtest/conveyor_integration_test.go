package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
	"voxelcraft.ai/internal/sim/world/logic/ids"
)

func newConveyorHarness(t *testing.T) *Harness {
	t.Helper()
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	return NewHarness(t, world.WorldConfig{
		ID:         "test",
		WorldType:  "OVERWORLD",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
	}, cats, "bot")
}

func mustPlace(t *testing.T, h *Harness, item string, pos world.Vec3i, taskID string) {
	t.Helper()
	h.AddInventory(item, 1)
	h.ClearAgentEvents()
	obs := h.Step(nil, []protocol.TaskReq{{
		ID:       taskID,
		Type:     "PLACE",
		ItemID:   item,
		BlockPos: pos.ToArray(),
	}}, nil)
	if code := actionResultCode(obs, taskID); code != "" {
		t.Fatalf("place %s failed: code=%q events=%v", item, code, obs.Events)
	}
}

func mustTransferSelfTo(t *testing.T, h *Harness, item string, count int, dstID string, taskID string) {
	t.Helper()
	h.AddInventory(item, count)
	h.ClearAgentEvents()
	obs := h.Step(nil, []protocol.TaskReq{{
		ID:     taskID,
		Type:   "TRANSFER",
		Src:    "SELF",
		Dst:    dstID,
		ItemID: item,
		Count:  count,
	}}, nil)
	if code := actionResultCode(obs, taskID); code != "" {
		t.Fatalf("transfer %s x%d to %s failed: code=%q events=%v", item, count, dstID, code, obs.Events)
	}
}

func hasTaskKind(obs protocol.ObsMsg, kind string) bool {
	for _, task := range obs.Tasks {
		if task.Kind == kind {
			return true
		}
	}
	return false
}

func waitTaskKindDone(t *testing.T, h *Harness, kind string, maxTicks int) {
	t.Helper()
	for i := 0; i < maxTicks; i++ {
		if !hasTaskKind(h.LastObs(), kind) {
			return
		}
		h.StepNoop()
	}
	t.Fatalf("task kind %s not done after %d ticks", kind, maxTicks)
}

func conveyorAt(s snapshot.SnapshotV1, pos world.Vec3i) *snapshot.ConveyorV1 {
	for i := range s.Conveyors {
		c := &s.Conveyors[i]
		if c.Pos == pos.ToArray() {
			return c
		}
	}
	return nil
}

func itemAtPos(s snapshot.SnapshotV1, pos world.Vec3i) *snapshot.ItemEntityV1 {
	for i := range s.Items {
		it := &s.Items[i]
		if it.Pos == pos.ToArray() {
			return it
		}
	}
	return nil
}

func itemByID(s snapshot.SnapshotV1, entityID string) *snapshot.ItemEntityV1 {
	for i := range s.Items {
		it := &s.Items[i]
		if it.EntityID == entityID {
			return it
		}
	}
	return nil
}

func containerAt(s snapshot.SnapshotV1, typ string, pos world.Vec3i) *snapshot.ContainerV1 {
	for i := range s.Containers {
		c := &s.Containers[i]
		if c.Type == typ && c.Pos == pos.ToArray() {
			return c
		}
	}
	return nil
}

func invCountMap(inv map[string]int, item string) int {
	if inv == nil {
		return 0
	}
	return inv[item]
}

func TestConveyor_MovesDroppedItemEntity(t *testing.T) {
	h := newConveyorHarness(t)
	self := h.LastObs().Self.Pos
	pos := world.Vec3i{X: self[0], Y: 0, Z: self[2]}
	clearArea(t, h, pos, 3)

	h.SetBlock(pos, "STONE")
	h.ClearAgentEvents()
	obs := h.Step(nil, []protocol.TaskReq{{ID: "K_mine", Type: "MINE", BlockPos: pos.ToArray()}}, nil)
	if code := actionResultCode(obs, "K_mine"); code != "" {
		t.Fatalf("start mine failed: %s events=%v", code, obs.Events)
	}
	waitTaskKindDone(t, h, "MINE", 30)

	_, snap := h.Snapshot()
	it := itemAtPos(snap, pos)
	if it == nil {
		t.Fatalf("expected dropped item at %+v after mine", pos)
	}
	itemID := it.EntityID

	mustPlace(t, h, "CONVEYOR", pos, "K_conv")
	_, snap = h.Snapshot()
	cv := conveyorAt(snap, pos)
	if cv == nil {
		t.Fatalf("missing conveyor at %+v", pos)
	}
	want := world.Vec3i{X: pos.X + cv.DX, Y: pos.Y, Z: pos.Z + cv.DZ}

	h.StepNoop()
	_, snap = h.Snapshot()
	moved := itemByID(snap, itemID)
	if moved == nil {
		t.Fatalf("missing item %s after conveyor tick", itemID)
	}
	if moved.Pos != want.ToArray() {
		t.Fatalf("item pos=%v want=%v", moved.Pos, want.ToArray())
	}
}

func TestConveyor_InsertsIntoChest(t *testing.T) {
	h := newConveyorHarness(t)
	self := h.LastObs().Self.Pos
	pos := world.Vec3i{X: self[0], Y: 0, Z: self[2]}
	clearArea(t, h, pos, 3)

	mustPlace(t, h, "CONVEYOR", pos, "K_conv")
	_, snap := h.Snapshot()
	cv := conveyorAt(snap, pos)
	if cv == nil {
		t.Fatalf("missing conveyor at %+v", pos)
	}
	back := world.Vec3i{X: pos.X - cv.DX, Y: 0, Z: pos.Z - cv.DZ}
	front := world.Vec3i{X: pos.X + cv.DX, Y: 0, Z: pos.Z + cv.DZ}

	mustPlace(t, h, "CHEST", back, "K_back")
	mustPlace(t, h, "CHEST", front, "K_front")

	backID := ids.ContainerID("CHEST", back.X, back.Y, back.Z)
	mustTransferSelfTo(t, h, "COAL", 1, backID, "K_transfer")

	h.StepNoop()
	h.StepNoop()

	_, snap = h.Snapshot()
	cb := containerAt(snap, "CHEST", back)
	cf := containerAt(snap, "CHEST", front)
	if cb == nil || cf == nil {
		t.Fatalf("expected chest containers: back=%v front=%v", cb != nil, cf != nil)
	}
	if got := invCountMap(cb.Inventory, "COAL"); got != 0 {
		t.Fatalf("back chest coal=%d want 0", got)
	}
	if got := invCountMap(cf.Inventory, "COAL"); got != 1 {
		t.Fatalf("front chest coal=%d want 1", got)
	}
}

func TestConveyor_PullsFromBackChest_AndMovesToFrontChest(t *testing.T) {
	h := newConveyorHarness(t)
	self := h.LastObs().Self.Pos
	pos := world.Vec3i{X: self[0], Y: 0, Z: self[2]}
	clearArea(t, h, pos, 3)

	mustPlace(t, h, "CONVEYOR", pos, "K_conv")
	_, snap := h.Snapshot()
	cv := conveyorAt(snap, pos)
	if cv == nil {
		t.Fatalf("missing conveyor at %+v", pos)
	}
	back := world.Vec3i{X: pos.X - cv.DX, Y: 0, Z: pos.Z - cv.DZ}
	front := world.Vec3i{X: pos.X + cv.DX, Y: 0, Z: pos.Z + cv.DZ}

	mustPlace(t, h, "CHEST", back, "K_back")
	mustPlace(t, h, "CHEST", front, "K_front")
	backID := ids.ContainerID("CHEST", back.X, back.Y, back.Z)
	mustTransferSelfTo(t, h, "COAL", 2, backID, "K_transfer")

	h.StepNoop()
	h.StepNoop()
	h.StepNoop()

	_, snap = h.Snapshot()
	cb := containerAt(snap, "CHEST", back)
	cf := containerAt(snap, "CHEST", front)
	if cb == nil || cf == nil {
		t.Fatalf("expected chest containers: back=%v front=%v", cb != nil, cf != nil)
	}
	if got := invCountMap(cb.Inventory, "COAL"); got != 0 {
		t.Fatalf("back chest coal=%d want 0", got)
	}
	if got := invCountMap(cf.Inventory, "COAL"); got != 2 {
		t.Fatalf("front chest coal=%d want 2", got)
	}
}

func TestConveyor_SensorGatesEnable(t *testing.T) {
	h := newConveyorHarness(t)
	self := h.LastObs().Self.Pos
	pos := world.Vec3i{X: self[0], Y: 0, Z: self[2]}
	clearArea(t, h, pos, 4)

	mustPlace(t, h, "CONVEYOR", pos, "K_conv")
	_, snap := h.Snapshot()
	cv := conveyorAt(snap, pos)
	if cv == nil {
		t.Fatalf("missing conveyor at %+v", pos)
	}
	back := world.Vec3i{X: pos.X - cv.DX, Y: 0, Z: pos.Z - cv.DZ}
	front := world.Vec3i{X: pos.X + cv.DX, Y: 0, Z: pos.Z + cv.DZ}
	right := world.Vec3i{X: cv.DZ, Y: 0, Z: -cv.DX}
	sensor := world.Vec3i{X: pos.X + right.X, Y: 0, Z: pos.Z + right.Z}
	dummy := world.Vec3i{X: sensor.X + right.X, Y: 0, Z: sensor.Z + right.Z}

	mustPlace(t, h, "CHEST", back, "K_back")
	mustPlace(t, h, "CHEST", front, "K_front")
	mustPlace(t, h, "CHEST", dummy, "K_dummy")
	mustPlace(t, h, "SENSOR", sensor, "K_sensor")
	backID := ids.ContainerID("CHEST", back.X, back.Y, back.Z)
	dummyID := ids.ContainerID("CHEST", dummy.X, dummy.Y, dummy.Z)

	mustTransferSelfTo(t, h, "COAL", 2, backID, "K_transfer_back")

	// sensor OFF (dummy chest empty): no movement
	h.StepNoop()
	h.StepNoop()
	h.StepNoop()
	_, snap = h.Snapshot()
	cb := containerAt(snap, "CHEST", back)
	cf := containerAt(snap, "CHEST", front)
	if cb == nil || cf == nil {
		t.Fatalf("expected chest containers: back=%v front=%v", cb != nil, cf != nil)
	}
	if got := invCountMap(cb.Inventory, "COAL"); got != 2 {
		t.Fatalf("back chest coal=%d want 2 when sensor off", got)
	}
	if got := invCountMap(cf.Inventory, "COAL"); got != 0 {
		t.Fatalf("front chest coal=%d want 0 when sensor off", got)
	}

	// sensor ON by non-empty adjacent chest
	mustTransferSelfTo(t, h, "STONE", 1, dummyID, "K_transfer_dummy")
	h.StepNoop()
	h.StepNoop()
	h.StepNoop()

	_, snap = h.Snapshot()
	cb = containerAt(snap, "CHEST", back)
	cf = containerAt(snap, "CHEST", front)
	if cb == nil || cf == nil {
		t.Fatalf("expected chest containers after sensor on")
	}
	if got := invCountMap(cb.Inventory, "COAL"); got != 0 {
		t.Fatalf("back chest coal=%d want 0 when sensor on", got)
	}
	if got := invCountMap(cf.Inventory, "COAL"); got != 2 {
		t.Fatalf("front chest coal=%d want 2 when sensor on", got)
	}
}

func TestConveyor_DisabledByAdjacentSwitchUntilToggledOn(t *testing.T) {
	h := newConveyorHarness(t)
	self := h.LastObs().Self.Pos
	pos := world.Vec3i{X: self[0], Y: 0, Z: self[2]}
	clearArea(t, h, pos, 4)

	// mine once to get an item entity at conveyor tile
	h.SetBlock(pos, "STONE")
	h.ClearAgentEvents()
	obs := h.Step(nil, []protocol.TaskReq{{ID: "K_mine", Type: "MINE", BlockPos: pos.ToArray()}}, nil)
	if code := actionResultCode(obs, "K_mine"); code != "" {
		t.Fatalf("start mine failed: %s events=%v", code, obs.Events)
	}
	waitTaskKindDone(t, h, "MINE", 30)
	_, snap := h.Snapshot()
	it := itemAtPos(snap, pos)
	if it == nil {
		t.Fatalf("expected dropped item at %+v", pos)
	}
	itemID := it.EntityID

	sw := world.Vec3i{X: pos.X + 1, Y: 0, Z: pos.Z}
	mustPlace(t, h, "SWITCH", sw, "K_switch")
	mustPlace(t, h, "CONVEYOR", pos, "K_conv")

	_, snap = h.Snapshot()
	cv := conveyorAt(snap, pos)
	if cv == nil {
		t.Fatalf("missing conveyor at %+v", pos)
	}
	want := world.Vec3i{X: pos.X + cv.DX, Y: 0, Z: pos.Z + cv.DZ}

	// switch defaults OFF => belt disabled
	h.StepNoop()
	_, snap = h.Snapshot()
	it = itemByID(snap, itemID)
	if it == nil {
		t.Fatalf("missing item %s", itemID)
	}
	if it.Pos != pos.ToArray() {
		t.Fatalf("item moved while switch off: got=%v want=%v", it.Pos, pos.ToArray())
	}

	h.SetAgentPos(sw)
	h.ClearAgentEvents()
	obs = h.Step([]protocol.InstantReq{{
		ID:       "I_toggle",
		Type:     "TOGGLE_SWITCH",
		TargetID: ids.SwitchIDAt(sw.X, sw.Y, sw.Z),
	}}, nil, nil)
	if code := actionResultCode(obs, "I_toggle"); code != "" {
		t.Fatalf("toggle switch failed: %s events=%v", code, obs.Events)
	}
	_, snap = h.Snapshot()
	it = itemByID(snap, itemID)
	if it == nil {
		t.Fatalf("missing item %s after toggle", itemID)
	}
	if it.Pos != want.ToArray() {
		t.Fatalf("item pos=%v want=%v", it.Pos, want.ToArray())
	}
}

func TestConveyor_EnabledViaWirePoweredByRemoteSwitch(t *testing.T) {
	h := newConveyorHarness(t)
	self := h.LastObs().Self.Pos
	conv := world.Vec3i{X: self[0], Y: 0, Z: self[2]}
	clearArea(t, h, conv, 6)

	// mine once to get an item entity at conveyor tile
	h.SetBlock(conv, "STONE")
	h.ClearAgentEvents()
	obs := h.Step(nil, []protocol.TaskReq{{ID: "K_mine", Type: "MINE", BlockPos: conv.ToArray()}}, nil)
	if code := actionResultCode(obs, "K_mine"); code != "" {
		t.Fatalf("start mine failed: %s events=%v", code, obs.Events)
	}
	waitTaskKindDone(t, h, "MINE", 30)
	_, snap := h.Snapshot()
	it := itemAtPos(snap, conv)
	if it == nil {
		t.Fatalf("expected dropped item at %+v", conv)
	}
	itemID := it.EntityID

	wire1 := world.Vec3i{X: conv.X + 1, Y: 0, Z: conv.Z}
	wire2 := world.Vec3i{X: conv.X + 2, Y: 0, Z: conv.Z}
	sw := world.Vec3i{X: conv.X + 3, Y: 0, Z: conv.Z}

	mustPlace(t, h, "WIRE", wire1, "K_wire1")
	mustPlace(t, h, "WIRE", wire2, "K_wire2")
	mustPlace(t, h, "SWITCH", sw, "K_switch")
	mustPlace(t, h, "CONVEYOR", conv, "K_conv")

	_, snap = h.Snapshot()
	cv := conveyorAt(snap, conv)
	if cv == nil {
		t.Fatalf("missing conveyor at %+v", conv)
	}
	want := world.Vec3i{X: conv.X + cv.DX, Y: 0, Z: conv.Z + cv.DZ}

	// remote switch OFF + adjacent wire present => disabled
	h.StepNoop()
	_, snap = h.Snapshot()
	it = itemByID(snap, itemID)
	if it == nil {
		t.Fatalf("missing item %s", itemID)
	}
	if it.Pos != conv.ToArray() {
		t.Fatalf("item moved while wire network unpowered: got=%v want=%v", it.Pos, conv.ToArray())
	}

	h.SetAgentPos(sw)
	h.ClearAgentEvents()
	obs = h.Step([]protocol.InstantReq{{
		ID:       "I_toggle",
		Type:     "TOGGLE_SWITCH",
		TargetID: ids.SwitchIDAt(sw.X, sw.Y, sw.Z),
	}}, nil, nil)
	if code := actionResultCode(obs, "I_toggle"); code != "" {
		t.Fatalf("toggle switch failed: %s events=%v", code, obs.Events)
	}
	_, snap = h.Snapshot()
	it = itemByID(snap, itemID)
	if it == nil {
		t.Fatalf("missing item %s after toggle", itemID)
	}
	if it.Pos != want.ToArray() {
		t.Fatalf("item pos=%v want=%v", it.Pos, want.ToArray())
	}
}

func TestSnapshotExportImport_ConveyorMetaRoundTrip(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	cfg := world.WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
	}

	h := NewHarness(t, cfg, cats, "bot")
	self := h.LastObs().Self.Pos
	pos := world.Vec3i{X: self[0], Y: 0, Z: self[2]}
	clearArea(t, h, pos, 2)
	mustPlace(t, h, "CONVEYOR", pos, "K_conv")

	snapTick := h.W.CurrentTick() - 1
	d1 := h.W.DebugStateDigest(snapTick)
	snap := h.W.ExportSnapshot(snapTick)
	before := conveyorAt(snap, pos)
	if before == nil {
		t.Fatalf("missing conveyor at %+v before export/import", pos)
	}

	w2, err := world.New(cfg, cats)
	if err != nil {
		t.Fatalf("world2: %v", err)
	}
	if err := w2.ImportSnapshot(snap); err != nil {
		t.Fatalf("import: %v", err)
	}
	d2 := w2.DebugStateDigest(snapTick)
	if d1 != d2 {
		t.Fatalf("digest mismatch after import: %s vs %s", d1, d2)
	}
	after := conveyorAt(w2.ExportSnapshot(snapTick), pos)
	if after == nil {
		t.Fatalf("missing conveyor at %+v after import", pos)
	}
	if after.DX != before.DX || after.DZ != before.DZ {
		t.Fatalf("conveyor dir changed after import: before=(%d,%d) after=(%d,%d)", before.DX, before.DZ, after.DX, after.DZ)
	}
}
