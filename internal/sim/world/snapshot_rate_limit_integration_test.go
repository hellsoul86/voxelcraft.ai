package world

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
)

func TestSnapshotExportImport_RateLimitWindows(t *testing.T) {
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

	out := make(chan []byte, 1)
	resp := make(chan JoinResponse, 1)
	w1.handleJoin(JoinRequest{Name: "bot", DeltaVoxels: false, Out: out, Resp: resp})
	jr := <-resp
	a := w1.agents[jr.Welcome.AgentID]
	if a == nil {
		t.Fatalf("missing agent")
	}

	nowTick := uint64(100)
	for i := 0; i < 5; i++ {
		ok, _ := a.RateLimitAllow("SAY", nowTick, 50, 5)
		if !ok {
			t.Fatalf("unexpected rate limit deny at i=%d", i)
		}
	}
	snap := w1.ExportSnapshot(nowTick)

	w2, err := New(cfg, cats)
	if err != nil {
		t.Fatalf("world2: %v", err)
	}
	if err := w2.ImportSnapshot(snap); err != nil {
		t.Fatalf("import: %v", err)
	}
	a2 := w2.agents[a.ID]
	if a2 == nil {
		t.Fatalf("missing imported agent")
	}

	rl := a2.RateWindowsSnapshot()
	rw, ok := rl["SAY"]
	if !ok {
		t.Fatalf("missing SAY rate window after import")
	}
	if got, want := rw.StartTick, nowTick; got != want {
		t.Fatalf("StartTick: got %d want %d", got, want)
	}
	if got, want := rw.Count, 5; got != want {
		t.Fatalf("Count: got %d want %d", got, want)
	}

	ok, cd := a2.RateLimitAllow("SAY", nowTick, 50, 5)
	if ok {
		t.Fatalf("expected rate limit deny after 5 actions")
	}
	if cd == 0 {
		t.Fatalf("expected non-zero cooldown")
	}
}
