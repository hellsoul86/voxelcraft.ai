package world

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
)

func TestSnapshotTreasuryByWorld_RoundTrip(t *testing.T) {
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
		Seed:       1,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("new world: %v", err)
	}

	w.orgs["ORG000001"] = &Organization{
		OrgID:       "ORG000001",
		Kind:        OrgGuild,
		Name:        "River",
		CreatedTick: 42,
		Members: map[string]OrgRole{
			"A1": OrgLeader,
		},
		TreasuryByWorld: map[string]map[string]int{
			"OVERWORLD": {"PLANK": 2},
			"MINE_L1":   {"COAL": 3},
		},
	}
	w.orgs["ORG000001"].Treasury = w.orgs["ORG000001"].TreasuryByWorld["OVERWORLD"]

	snap := w.ExportSnapshot(77)
	if len(snap.Orgs) != 1 {
		t.Fatalf("org snapshots: got %d want 1", len(snap.Orgs))
	}
	os := snap.Orgs[0]
	if got := os.Treasury["PLANK"]; got != 2 {
		t.Fatalf("snapshot current-world treasury mismatch: got %d want 2", got)
	}
	if got := os.TreasuryByWorld["MINE_L1"]["COAL"]; got != 3 {
		t.Fatalf("snapshot treasury_by_world mismatch: got %d want 3", got)
	}

	w2, err := New(WorldConfig{
		ID:         "OVERWORLD",
		TickRateHz: 5,
		DayTicks:   6000,
		ObsRadius:  7,
		Height:     1,
		Seed:       1,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("new world2: %v", err)
	}
	if err := w2.ImportSnapshot(snap); err != nil {
		t.Fatalf("import snapshot: %v", err)
	}
	org := w2.orgs["ORG000001"]
	if org == nil {
		t.Fatalf("org missing after import")
	}
	if got := org.TreasuryByWorld["OVERWORLD"]["PLANK"]; got != 2 {
		t.Fatalf("import OVERWORLD treasury mismatch: got %d want 2", got)
	}
	if got := org.TreasuryByWorld["MINE_L1"]["COAL"]; got != 3 {
		t.Fatalf("import MINE_L1 treasury mismatch: got %d want 3", got)
	}
	if got := org.Treasury["PLANK"]; got != 2 {
		t.Fatalf("import active-world treasury view mismatch: got %d want 2", got)
	}
}
