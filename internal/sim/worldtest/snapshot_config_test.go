package worldtest

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
	world "voxelcraft.ai/internal/sim/world"
)

func TestSnapshotImport_RestoresOperationalConfig(t *testing.T) {
	cats, err := catalogs.Load("../../../configs")
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	cfg := world.WorldConfig{
		ID:                "test",
		TickRateHz:        5,
		DayTicks:          6000,
		SeasonLengthTicks: 18000,
		ObsRadius:         7,
		Height:            1,
		Seed:              42,
		BoundaryR:         4000,

		BiomeRegionSize:                 80,
		SpawnClearRadius:                7,
		OreClusterProbScalePermille:     1200,
		TerrainClusterProbScalePermille: 900,
		SprinkleStonePermille:           9,
		SprinkleDirtPermille:            3,
		SprinkleLogPermille:             1,

		SnapshotEveryTicks: 123,
		DirectorEveryTicks: 456,
		RateLimits: world.RateLimitConfig{
			SayWindowTicks:        7,
			SayMax:                2,
			MarketSayWindowTicks:  8,
			MarketSayMax:          1,
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
	w1, err := world.New(cfg, cats)
	if err != nil {
		t.Fatalf("world1: %v", err)
	}
	snap := w1.ExportSnapshot(0)

	cfg2 := cfg
	cfg2.SnapshotEveryTicks = 999
	cfg2.DirectorEveryTicks = 999
	cfg2.SeasonLengthTicks = 999
	cfg2.BiomeRegionSize = 999
	cfg2.SpawnClearRadius = 99
	cfg2.OreClusterProbScalePermille = 999
	cfg2.TerrainClusterProbScalePermille = 999
	cfg2.SprinkleStonePermille = 99
	cfg2.SprinkleDirtPermille = 99
	cfg2.SprinkleLogPermille = 99
	cfg2.RateLimits.SayWindowTicks = 99
	cfg2.RateLimits.SayMax = 99
	cfg2.RateLimits.MarketSayWindowTicks = 99
	cfg2.RateLimits.MarketSayMax = 99
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

	w2, err := world.New(cfg2, cats)
	if err != nil {
		t.Fatalf("world2: %v", err)
	}
	if err := w2.ImportSnapshot(snap); err != nil {
		t.Fatalf("import: %v", err)
	}

	got := w2.Config()
	if got.SnapshotEveryTicks != cfg.SnapshotEveryTicks {
		t.Fatalf("SnapshotEveryTicks: got %d want %d", got.SnapshotEveryTicks, cfg.SnapshotEveryTicks)
	}
	if got.DirectorEveryTicks != cfg.DirectorEveryTicks {
		t.Fatalf("DirectorEveryTicks: got %d want %d", got.DirectorEveryTicks, cfg.DirectorEveryTicks)
	}
	if got.SeasonLengthTicks != cfg.SeasonLengthTicks {
		t.Fatalf("SeasonLengthTicks: got %d want %d", got.SeasonLengthTicks, cfg.SeasonLengthTicks)
	}
	if got.BiomeRegionSize != cfg.BiomeRegionSize {
		t.Fatalf("BiomeRegionSize: got %d want %d", got.BiomeRegionSize, cfg.BiomeRegionSize)
	}
	if got.SpawnClearRadius != cfg.SpawnClearRadius {
		t.Fatalf("SpawnClearRadius: got %d want %d", got.SpawnClearRadius, cfg.SpawnClearRadius)
	}
	if got.OreClusterProbScalePermille != cfg.OreClusterProbScalePermille {
		t.Fatalf("OreClusterProbScalePermille: got %d want %d", got.OreClusterProbScalePermille, cfg.OreClusterProbScalePermille)
	}
	if got.TerrainClusterProbScalePermille != cfg.TerrainClusterProbScalePermille {
		t.Fatalf("TerrainClusterProbScalePermille: got %d want %d", got.TerrainClusterProbScalePermille, cfg.TerrainClusterProbScalePermille)
	}
	if got.SprinkleStonePermille != cfg.SprinkleStonePermille {
		t.Fatalf("SprinkleStonePermille: got %d want %d", got.SprinkleStonePermille, cfg.SprinkleStonePermille)
	}
	if got.SprinkleDirtPermille != cfg.SprinkleDirtPermille {
		t.Fatalf("SprinkleDirtPermille: got %d want %d", got.SprinkleDirtPermille, cfg.SprinkleDirtPermille)
	}
	if got.SprinkleLogPermille != cfg.SprinkleLogPermille {
		t.Fatalf("SprinkleLogPermille: got %d want %d", got.SprinkleLogPermille, cfg.SprinkleLogPermille)
	}
	if got.RateLimits.SayWindowTicks != cfg.RateLimits.SayWindowTicks {
		t.Fatalf("RateLimits.SayWindowTicks: got %d want %d", got.RateLimits.SayWindowTicks, cfg.RateLimits.SayWindowTicks)
	}
	if got.RateLimits.SayMax != cfg.RateLimits.SayMax {
		t.Fatalf("RateLimits.SayMax: got %d want %d", got.RateLimits.SayMax, cfg.RateLimits.SayMax)
	}
	if got.RateLimits.MarketSayWindowTicks != cfg.RateLimits.MarketSayWindowTicks {
		t.Fatalf("RateLimits.MarketSayWindowTicks: got %d want %d", got.RateLimits.MarketSayWindowTicks, cfg.RateLimits.MarketSayWindowTicks)
	}
	if got.RateLimits.MarketSayMax != cfg.RateLimits.MarketSayMax {
		t.Fatalf("RateLimits.MarketSayMax: got %d want %d", got.RateLimits.MarketSayMax, cfg.RateLimits.MarketSayMax)
	}
	if got.RateLimits.WhisperWindowTicks != cfg.RateLimits.WhisperWindowTicks {
		t.Fatalf("RateLimits.WhisperWindowTicks: got %d want %d", got.RateLimits.WhisperWindowTicks, cfg.RateLimits.WhisperWindowTicks)
	}
	if got.RateLimits.WhisperMax != cfg.RateLimits.WhisperMax {
		t.Fatalf("RateLimits.WhisperMax: got %d want %d", got.RateLimits.WhisperMax, cfg.RateLimits.WhisperMax)
	}
	if got.RateLimits.OfferTradeWindowTicks != cfg.RateLimits.OfferTradeWindowTicks {
		t.Fatalf("RateLimits.OfferTradeWindowTicks: got %d want %d", got.RateLimits.OfferTradeWindowTicks, cfg.RateLimits.OfferTradeWindowTicks)
	}
	if got.RateLimits.OfferTradeMax != cfg.RateLimits.OfferTradeMax {
		t.Fatalf("RateLimits.OfferTradeMax: got %d want %d", got.RateLimits.OfferTradeMax, cfg.RateLimits.OfferTradeMax)
	}
	if got.RateLimits.PostBoardWindowTicks != cfg.RateLimits.PostBoardWindowTicks {
		t.Fatalf("RateLimits.PostBoardWindowTicks: got %d want %d", got.RateLimits.PostBoardWindowTicks, cfg.RateLimits.PostBoardWindowTicks)
	}
	if got.RateLimits.PostBoardMax != cfg.RateLimits.PostBoardMax {
		t.Fatalf("RateLimits.PostBoardMax: got %d want %d", got.RateLimits.PostBoardMax, cfg.RateLimits.PostBoardMax)
	}
	if got.LawNoticeTicks != cfg.LawNoticeTicks {
		t.Fatalf("LawNoticeTicks: got %d want %d", got.LawNoticeTicks, cfg.LawNoticeTicks)
	}
	if got.LawVoteTicks != cfg.LawVoteTicks {
		t.Fatalf("LawVoteTicks: got %d want %d", got.LawVoteTicks, cfg.LawVoteTicks)
	}
	if got.BlueprintAutoPullRange != cfg.BlueprintAutoPullRange {
		t.Fatalf("BlueprintAutoPullRange: got %d want %d", got.BlueprintAutoPullRange, cfg.BlueprintAutoPullRange)
	}
	if got.BlueprintBlocksPerTick != cfg.BlueprintBlocksPerTick {
		t.Fatalf("BlueprintBlocksPerTick: got %d want %d", got.BlueprintBlocksPerTick, cfg.BlueprintBlocksPerTick)
	}
	if got.AccessPassCoreRadius != cfg.AccessPassCoreRadius {
		t.Fatalf("AccessPassCoreRadius: got %d want %d", got.AccessPassCoreRadius, cfg.AccessPassCoreRadius)
	}
	if got.FunDecayWindowTicks != cfg.FunDecayWindowTicks {
		t.Fatalf("FunDecayWindowTicks: got %d want %d", got.FunDecayWindowTicks, cfg.FunDecayWindowTicks)
	}
	if got.FunDecayBase != cfg.FunDecayBase {
		t.Fatalf("FunDecayBase: got %v want %v", got.FunDecayBase, cfg.FunDecayBase)
	}
	if got.StructureSurvivalTicks != cfg.StructureSurvivalTicks {
		t.Fatalf("StructureSurvivalTicks: got %d want %d", got.StructureSurvivalTicks, cfg.StructureSurvivalTicks)
	}
	if got.MaintenanceCost["IRON_INGOT"] != cfg.MaintenanceCost["IRON_INGOT"] {
		t.Fatalf("MaintenanceCost[IRON_INGOT]: got %d want %d", got.MaintenanceCost["IRON_INGOT"], cfg.MaintenanceCost["IRON_INGOT"])
	}
	if got.MaintenanceCost["COAL"] != cfg.MaintenanceCost["COAL"] {
		t.Fatalf("MaintenanceCost[COAL]: got %d want %d", got.MaintenanceCost["COAL"], cfg.MaintenanceCost["COAL"])
	}
}
