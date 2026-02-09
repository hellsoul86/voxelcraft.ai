package world

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
)

func TestSnapshotImport_RestoresOperationalConfig(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	cfg := WorldConfig{
		ID:         "test",
		TickRateHz: 5,
		DayTicks:   6000,
		SeasonLengthTicks: 18000,
		ObsRadius:  7,
		Height:     64,
		Seed:       42,
		BoundaryR:  4000,

		SnapshotEveryTicks: 123,
		DirectorEveryTicks: 456,
		RateLimits: RateLimitConfig{
			SayWindowTicks:        7,
			SayMax:                2,
			WhisperWindowTicks:    9,
			WhisperMax:            3,
			OfferTradeWindowTicks: 11,
			OfferTradeMax:         4,
			PostBoardWindowTicks:  13,
			PostBoardMax:          5,
		},

		LawNoticeTicks: 111,
		LawVoteTicks:   222,

		BlueprintAutoPullRange: 33,
		BlueprintBlocksPerTick: 4,

		AccessPassCoreRadius: 17,
		MaintenanceCost: map[string]int{
			"IRON_INGOT": 2,
			"COAL":       3,
		},

		FunDecayWindowTicks:    314,
		FunDecayBase:           0.8,
		StructureSurvivalTicks: 2718,
	}
	w1, err := New(cfg, cats)
	if err != nil {
		t.Fatalf("world1: %v", err)
	}
	snap := w1.ExportSnapshot(0)

	cfg2 := cfg
	cfg2.SnapshotEveryTicks = 999
	cfg2.DirectorEveryTicks = 999
	cfg2.SeasonLengthTicks = 999
	cfg2.RateLimits.SayWindowTicks = 99
	cfg2.RateLimits.SayMax = 99
	cfg2.RateLimits.WhisperWindowTicks = 99
	cfg2.RateLimits.WhisperMax = 99
	cfg2.RateLimits.OfferTradeWindowTicks = 99
	cfg2.RateLimits.OfferTradeMax = 99
	cfg2.RateLimits.PostBoardWindowTicks = 99
	cfg2.RateLimits.PostBoardMax = 99
	cfg2.LawNoticeTicks = 999
	cfg2.LawVoteTicks = 999
	cfg2.BlueprintAutoPullRange = 999
	cfg2.BlueprintBlocksPerTick = 999
	cfg2.AccessPassCoreRadius = 999
	cfg2.MaintenanceCost = map[string]int{"IRON_INGOT": 99}
	cfg2.FunDecayWindowTicks = 999
	cfg2.FunDecayBase = 0.99
	cfg2.StructureSurvivalTicks = 999

	w2, err := New(cfg2, cats)
	if err != nil {
		t.Fatalf("world2: %v", err)
	}
	if err := w2.ImportSnapshot(snap); err != nil {
		t.Fatalf("import: %v", err)
	}

	if got, want := w2.cfg.SnapshotEveryTicks, cfg.SnapshotEveryTicks; got != want {
		t.Fatalf("SnapshotEveryTicks: got %d want %d", got, want)
	}
	if got, want := w2.cfg.DirectorEveryTicks, cfg.DirectorEveryTicks; got != want {
		t.Fatalf("DirectorEveryTicks: got %d want %d", got, want)
	}
	if got, want := w2.cfg.SeasonLengthTicks, cfg.SeasonLengthTicks; got != want {
		t.Fatalf("SeasonLengthTicks: got %d want %d", got, want)
	}
	if got, want := w2.cfg.RateLimits.SayWindowTicks, cfg.RateLimits.SayWindowTicks; got != want {
		t.Fatalf("RateLimits.SayWindowTicks: got %d want %d", got, want)
	}
	if got, want := w2.cfg.RateLimits.SayMax, cfg.RateLimits.SayMax; got != want {
		t.Fatalf("RateLimits.SayMax: got %d want %d", got, want)
	}
	if got, want := w2.cfg.RateLimits.WhisperWindowTicks, cfg.RateLimits.WhisperWindowTicks; got != want {
		t.Fatalf("RateLimits.WhisperWindowTicks: got %d want %d", got, want)
	}
	if got, want := w2.cfg.RateLimits.WhisperMax, cfg.RateLimits.WhisperMax; got != want {
		t.Fatalf("RateLimits.WhisperMax: got %d want %d", got, want)
	}
	if got, want := w2.cfg.RateLimits.OfferTradeWindowTicks, cfg.RateLimits.OfferTradeWindowTicks; got != want {
		t.Fatalf("RateLimits.OfferTradeWindowTicks: got %d want %d", got, want)
	}
	if got, want := w2.cfg.RateLimits.OfferTradeMax, cfg.RateLimits.OfferTradeMax; got != want {
		t.Fatalf("RateLimits.OfferTradeMax: got %d want %d", got, want)
	}
	if got, want := w2.cfg.RateLimits.PostBoardWindowTicks, cfg.RateLimits.PostBoardWindowTicks; got != want {
		t.Fatalf("RateLimits.PostBoardWindowTicks: got %d want %d", got, want)
	}
	if got, want := w2.cfg.RateLimits.PostBoardMax, cfg.RateLimits.PostBoardMax; got != want {
		t.Fatalf("RateLimits.PostBoardMax: got %d want %d", got, want)
	}
	if got, want := w2.cfg.LawNoticeTicks, cfg.LawNoticeTicks; got != want {
		t.Fatalf("LawNoticeTicks: got %d want %d", got, want)
	}
	if got, want := w2.cfg.LawVoteTicks, cfg.LawVoteTicks; got != want {
		t.Fatalf("LawVoteTicks: got %d want %d", got, want)
	}
	if got, want := w2.cfg.BlueprintAutoPullRange, cfg.BlueprintAutoPullRange; got != want {
		t.Fatalf("BlueprintAutoPullRange: got %d want %d", got, want)
	}
	if got, want := w2.cfg.BlueprintBlocksPerTick, cfg.BlueprintBlocksPerTick; got != want {
		t.Fatalf("BlueprintBlocksPerTick: got %d want %d", got, want)
	}
	if got, want := w2.cfg.AccessPassCoreRadius, cfg.AccessPassCoreRadius; got != want {
		t.Fatalf("AccessPassCoreRadius: got %d want %d", got, want)
	}
	if got, want := w2.cfg.FunDecayWindowTicks, cfg.FunDecayWindowTicks; got != want {
		t.Fatalf("FunDecayWindowTicks: got %d want %d", got, want)
	}
	if got, want := w2.cfg.FunDecayBase, cfg.FunDecayBase; got != want {
		t.Fatalf("FunDecayBase: got %v want %v", got, want)
	}
	if got, want := w2.cfg.StructureSurvivalTicks, cfg.StructureSurvivalTicks; got != want {
		t.Fatalf("StructureSurvivalTicks: got %d want %d", got, want)
	}
	if got, want := w2.cfg.MaintenanceCost["IRON_INGOT"], cfg.MaintenanceCost["IRON_INGOT"]; got != want {
		t.Fatalf("MaintenanceCost[IRON_INGOT]: got %d want %d", got, want)
	}
	if got, want := w2.cfg.MaintenanceCost["COAL"], cfg.MaintenanceCost["COAL"]; got != want {
		t.Fatalf("MaintenanceCost[COAL]: got %d want %d", got, want)
	}
}
