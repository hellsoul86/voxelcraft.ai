package snapshot

import (
	"strings"

	snapv1 "voxelcraft.ai/internal/persistence/snapshot"
)

type ConfigPatch struct {
	BiomeRegionSize                 int
	SpawnClearRadius                int
	OreClusterProbScalePermille     int
	TerrainClusterProbScalePermille int
	SprinkleStonePermille           int
	SprinkleDirtPermille            int
	SprinkleLogPermille             int

	StarterItems    map[string]int
	HasStarterItems bool

	SnapshotEveryTicks int
	DirectorEveryTicks int

	RateLimits    snapv1.RateLimitsV1
	HasRateLimits bool

	LawNoticeTicks int
	LawVoteTicks   int

	BlueprintAutoPullRange int
	BlueprintBlocksPerTick int

	AccessPassCoreRadius int
	MaintenanceCost      map[string]int
	HasMaintenanceCost   bool

	FunDecayWindowTicks    int
	FunDecayBase           float64
	StructureSurvivalTicks int
}

type RuntimePatch struct {
	Weather           string
	WeatherUntilTick  uint64
	ActiveEventID     string
	ActiveEventStart  uint64
	ActiveEventEnds   uint64
	ActiveEventCenter [3]int
	ActiveEventRadius int
}

func BuildConfigPatch(s snapv1.SnapshotV1) ConfigPatch {
	patch := ConfigPatch{
		BiomeRegionSize:                 positiveOrZero(s.BiomeRegionSize),
		SpawnClearRadius:                positiveOrZero(s.SpawnClearRadius),
		OreClusterProbScalePermille:     positiveOrZero(s.OreClusterProbScalePermille),
		TerrainClusterProbScalePermille: positiveOrZero(s.TerrainClusterProbScalePermille),
		SprinkleStonePermille:           positiveOrZero(s.SprinkleStonePermille),
		SprinkleDirtPermille:            positiveOrZero(s.SprinkleDirtPermille),
		SprinkleLogPermille:             positiveOrZero(s.SprinkleLogPermille),
		SnapshotEveryTicks:              positiveOrZero(s.SnapshotEveryTicks),
		DirectorEveryTicks:              positiveOrZero(s.DirectorEveryTicks),
		LawNoticeTicks:                  positiveOrZero(s.LawNoticeTicks),
		LawVoteTicks:                    positiveOrZero(s.LawVoteTicks),
		BlueprintAutoPullRange:          positiveOrZero(s.BlueprintAutoPullRange),
		BlueprintBlocksPerTick:          positiveOrZero(s.BlueprintBlocksPerTick),
		AccessPassCoreRadius:            positiveOrZero(s.AccessPassCoreRadius),
		FunDecayWindowTicks:             positiveOrZero(s.FunDecayWindowTicks),
		FunDecayBase:                    positiveFloatOrZero(s.FunDecayBase),
		StructureSurvivalTicks:          positiveOrZero(s.StructureSurvivalTicks),
	}

	if s.StarterItems != nil {
		patch.HasStarterItems = true
		patch.StarterItems = map[string]int{}
		for item, n := range s.StarterItems {
			if strings.TrimSpace(item) == "" || n <= 0 {
				continue
			}
			patch.StarterItems[item] = n
		}
	}

	if s.RateLimits.SayWindowTicks > 0 ||
		s.RateLimits.SayMax > 0 ||
		s.RateLimits.MarketSayWindowTicks > 0 ||
		s.RateLimits.MarketSayMax > 0 ||
		s.RateLimits.WhisperWindowTicks > 0 ||
		s.RateLimits.WhisperMax > 0 ||
		s.RateLimits.OfferTradeWindowTicks > 0 ||
		s.RateLimits.OfferTradeMax > 0 ||
		s.RateLimits.PostBoardWindowTicks > 0 ||
		s.RateLimits.PostBoardMax > 0 {
		patch.HasRateLimits = true
		patch.RateLimits = s.RateLimits
	}

	if len(s.MaintenanceCost) > 0 {
		patch.HasMaintenanceCost = true
		patch.MaintenanceCost = map[string]int{}
		for item, n := range s.MaintenanceCost {
			if item != "" && n > 0 {
				patch.MaintenanceCost[item] = n
			}
		}
	}

	return patch
}

func BuildRuntimePatch(s snapv1.SnapshotV1) RuntimePatch {
	return RuntimePatch{
		Weather:           s.Weather,
		WeatherUntilTick:  s.WeatherUntilTick,
		ActiveEventID:     s.ActiveEventID,
		ActiveEventStart:  s.ActiveEventStart,
		ActiveEventEnds:   s.ActiveEventEnds,
		ActiveEventCenter: s.ActiveEventCenter,
		ActiveEventRadius: s.ActiveEventRadius,
	}
}

func positiveOrZero(v int) int {
	if v > 0 {
		return v
	}
	return 0
}

func positiveFloatOrZero(v float64) float64 {
	if v > 0 {
		return v
	}
	return 0
}
