package world

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
)

func TestDirector_FirstWeekSchedule(t *testing.T) {
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
		Seed:       1,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	w.systemDirector(0)
	if got, want := w.activeEventID, "MARKET_WEEK"; got != want {
		t.Fatalf("day1 event: got %s want %s", got, want)
	}

	// At day2 boundary, day1 event expires and day2 starts.
	w.systemDirector(6000)
	if got, want := w.activeEventID, "CRYSTAL_RIFT"; got != want {
		t.Fatalf("day2 event: got %s want %s", got, want)
	}
}

func TestDirectorMetrics_Basic(t *testing.T) {
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
		Seed:       1,
		BoundaryR:  4000,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	// Add two agents.
	w.joinAgent("a1", false, nil)
	w.joinAgent("a2", false, nil)

	// Simulate some stats.
	for i := 0; i < 10; i++ {
		w.stats.RecordTrade(0)
	}
	for i := 0; i < 5; i++ {
		w.stats.RecordDenied(0)
	}

	m := w.computeDirectorMetrics(0)
	if m.Trade < 0.99 {
		t.Fatalf("expected trade metric near 1, got %f", m.Trade)
	}
	if m.Conflict <= 0 {
		t.Fatalf("expected conflict > 0, got %f", m.Conflict)
	}
}
