package world

import (
	"testing"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestSign_SnapshotRoundTrip_PreservesTextAndDigest(t *testing.T) {
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

	resp := make(chan JoinResponse, 1)
	w1.handleJoin(JoinRequest{Name: "writer", DeltaVoxels: false, Out: nil, Resp: resp})
	j := <-resp
	a := w1.agents[j.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	signPos := Vec3i{X: a.Pos.X + 1, Y: a.Pos.Y, Z: a.Pos.Z}
	sid := w1.catalogs.Blocks.Index["SIGN"]
	w1.chunks.SetBlock(signPos, sid)
	a.Pos = signPos
	w1.applyInstant(a, protocol.InstantReq{
		ID:       "I_set",
		Type:     "SET_SIGN",
		TargetID: signIDAt(signPos),
		Text:     "persist me",
	}, 0)

	// Advance a few ticks.
	for i := 0; i < 5; i++ {
		w1.step(nil, nil, nil)
	}

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
	if s := w2.signs[signPos]; s == nil || s.Text != "persist me" {
		t.Fatalf("sign not restored: %+v", s)
	}
}
