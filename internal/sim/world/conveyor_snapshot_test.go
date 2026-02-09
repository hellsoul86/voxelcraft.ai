package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestSnapshotExportImport_ConveyorMetaRoundTrip(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	cfg := WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     64,
		Seed:       42,
		BoundaryR:  4000,
	}

	w1, err := New(cfg, cats)
	if err != nil {
		t.Fatalf("world1: %v", err)
	}

	// Join an agent and place a conveyor (with non-default direction).
	respCh := make(chan JoinResponse, 1)
	w1.step([]JoinRequest{{Name: "bot", Resp: respCh}}, nil, nil)
	jr := <-respCh
	a := w1.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}
	a.Inventory["CONVEYOR"] = 1
	a.Yaw = 180 // -Z

	pos := Vec3i{X: a.Pos.X, Y: w1.cfg.Height - 2, Z: a.Pos.Z}
	act := protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            w1.CurrentTick(),
		Tasks: []protocol.TaskReq{
			{ID: "K1", Type: "PLACE", ItemID: "CONVEYOR", BlockPos: pos.ToArray()},
		},
	}
	w1.step(nil, nil, []ActionEnvelope{{AgentID: a.ID, Act: act}})

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

	if got := w2.conveyors[pos]; !(got.DX == 0 && got.DZ == -1) {
		t.Fatalf("imported conveyor meta=%+v want dx=0 dz=-1", got)
	}
}
