package world

import (
	"testing"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/catalogs"
)

func TestHandleAdminResetRequests_ResetsWorld(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "OVERWORLD",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       77,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("new world: %v", err)
	}

	w.tick.Store(10)
	w.claims["LAND_1"] = &LandClaim{LandID: "LAND_1", Owner: "A1", Anchor: Vec3i{X: 0, Y: 0, Z: 0}, Radius: 8}
	w.orgs["ORG000001"] = &Organization{
		OrgID:           "ORG000001",
		Kind:            OrgGuild,
		Name:            "Guild",
		Members:         map[string]OrgRole{"A1": OrgLeader},
		Treasury:        map[string]int{"PLANK": 2},
		TreasuryByWorld: map[string]map[string]int{"OVERWORLD": {"PLANK": 2}},
	}
	w.snapshotSink = make(chan snapshot.SnapshotV1, 1)

	respCh := make(chan adminResetResp, 1)
	w.handleAdminResetRequests([]adminResetReq{{Resp: respCh}})
	resp := <-respCh
	if resp.Err != "" {
		t.Fatalf("unexpected reset error: %s", resp.Err)
	}
	if resp.Tick != 10 {
		t.Fatalf("reset tick mismatch: got %d want 10", resp.Tick)
	}
	if len(w.claims) != 0 {
		t.Fatalf("expected claims cleared on reset, got %d", len(w.claims))
	}
	if w.cfg.Seed != 78 {
		t.Fatalf("expected seed increment, got %d", w.cfg.Seed)
	}
	if w.resetTotal != 1 {
		t.Fatalf("expected resetTotal=1, got %d", w.resetTotal)
	}
	org := w.orgs["ORG000001"]
	if org == nil {
		t.Fatalf("expected org to persist")
	}
	if got := org.TreasuryByWorld["OVERWORLD"]["PLANK"]; got != 0 {
		t.Fatalf("expected world treasury reset, got %d", got)
	}
}

func TestHandleAdminResetRequests_BackpressureDoesNotReset(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "OVERWORLD",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       9,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("new world: %v", err)
	}
	w.tick.Store(3)
	w.claims["LAND_1"] = &LandClaim{LandID: "LAND_1", Owner: "A1", Anchor: Vec3i{X: 0, Y: 0, Z: 0}, Radius: 8}
	w.snapshotSink = make(chan snapshot.SnapshotV1) // unbuffered, no receiver => backpressure

	respCh := make(chan adminResetResp, 1)
	w.handleAdminResetRequests([]adminResetReq{{Resp: respCh}})
	resp := <-respCh
	if resp.Err == "" {
		t.Fatalf("expected reset error under snapshot sink backpressure")
	}
	if len(w.claims) != 1 {
		t.Fatalf("reset should not run on backpressure; claims=%d", len(w.claims))
	}
	if w.resetTotal != 0 {
		t.Fatalf("resetTotal should remain 0, got %d", w.resetTotal)
	}
}
