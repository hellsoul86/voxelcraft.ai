package world

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
)

func TestSnapshotExportImport_AgentMemory(t *testing.T) {
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

	a.MemorySave("keep", "v", 100, 0)  // expires at tick 100
	a.MemorySave("expired", "x", 1, 0) // expires at tick 1

	snap := w1.ExportSnapshot(1)

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

	if got, ok := a2.Memory["keep"]; !ok || got.Value != "v" || got.ExpiryTick != 100 {
		t.Fatalf("keep memory not restored: ok=%v got=%+v", ok, got)
	}
	if _, ok := a2.Memory["expired"]; ok {
		t.Fatalf("expected expired key to be filtered on snapshot export/import")
	}
}
