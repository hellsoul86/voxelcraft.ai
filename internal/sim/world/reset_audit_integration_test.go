package world

import (
	"testing"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/catalogs"
)

type memAudit struct {
	entries []AuditEntry
}

func (m *memAudit) WriteAudit(e AuditEntry) error {
	m.entries = append(m.entries, e)
	return nil
}

func TestAdminReset_WritesWorldResetAudit(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "MINE_L1",
		WorldType:  "MINE_L1",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       9,
		BoundaryR:  400,
	}, cats)
	if err != nil {
		t.Fatalf("new world: %v", err)
	}
	aud := &memAudit{}
	w.SetAuditLogger(aud)
	w.snapshotSink = make(chan snapshot.SnapshotV1, 1)
	w.tick.Store(15)

	respCh := make(chan adminResetResp, 1)
	w.handleAdminResetRequests([]adminResetReq{{Resp: respCh}})
	resp := <-respCh
	if resp.Err != "" {
		t.Fatalf("unexpected reset error: %v", resp.Err)
	}

	found := false
	for _, e := range aud.entries {
		if e.Action != "WORLD_RESET" {
			continue
		}
		if e.Reason == "ADMIN_RESET" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected ADMIN_RESET world reset audit entry, got %+v", aud.entries)
	}
}
