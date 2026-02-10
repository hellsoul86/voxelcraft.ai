package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestSnapshotExportImport_SwitchStateRoundTrip(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	cfg := WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       42,
		BoundaryR:  4000,
	}

	w1, err := New(cfg, cats)
	if err != nil {
		t.Fatalf("world1: %v", err)
	}

	respCh := make(chan JoinResponse, 1)
	w1.step([]JoinRequest{{Name: "bot", Resp: respCh}}, nil, nil)
	jr := <-respCh
	a := w1.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	pos := Vec3i{X: a.Pos.X, Y: 0, Z: a.Pos.Z}
	w1.chunks.SetBlock(pos, w1.chunks.gen.Air)
	a.Inventory["SWITCH"] = 1
	w1.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w1.CurrentTick(),
		Tasks:           []protocol.TaskReq{{ID: "K1", Type: "PLACE", ItemID: "SWITCH", BlockPos: pos.ToArray()}},
	}}})
	a.Pos = pos
	w1.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w1.CurrentTick(),
		Instants:        []protocol.InstantReq{{ID: "I1", Type: "TOGGLE_SWITCH", TargetID: switchIDAt(pos)}},
	}}})

	snapTick := w1.CurrentTick() - 1
	d1 := w1.stateDigest(snapTick)
	snap := w1.ExportSnapshot(snapTick)

	w2, err := New(cfg, cats)
	if err != nil {
		t.Fatalf("world2: %v", err)
	}
	if err := w2.ImportSnapshot(snap); err != nil {
		t.Fatalf("import: %v", err)
	}
	d2 := w2.stateDigest(snapTick)
	if d1 != d2 {
		t.Fatalf("digest mismatch after import: %s vs %s", d1, d2)
	}
	if !w2.switches[pos] {
		t.Fatalf("expected switch to be on after import")
	}
}
