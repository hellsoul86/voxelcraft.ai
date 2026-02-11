package world

import (
	"fmt"
	"strings"

	"voxelcraft.ai/internal/persistence/snapshot"
)

func (w *World) validateSnapshotImport(s snapshot.SnapshotV1) error {
	if s.Header.Version != 1 {
		return fmt.Errorf("unsupported snapshot version: %d", s.Header.Version)
	}
	if w.cfg.Seed != s.Seed {
		return fmt.Errorf("snapshot seed mismatch: cfg=%d snap=%d", w.cfg.Seed, s.Seed)
	}
	if w.cfg.Height != s.Height {
		return fmt.Errorf("snapshot height mismatch: cfg=%d snap=%d", w.cfg.Height, s.Height)
	}
	if s.Height != 1 {
		return fmt.Errorf("unsupported snapshot height for 2D world: height=%d", s.Height)
	}
	if w.cfg.DayTicks != s.DayTicks {
		return fmt.Errorf("snapshot day_ticks mismatch: cfg=%d snap=%d", w.cfg.DayTicks, s.DayTicks)
	}
	if w.cfg.ObsRadius != s.ObsRadius {
		return fmt.Errorf("snapshot obs_radius mismatch: cfg=%d snap=%d", w.cfg.ObsRadius, s.ObsRadius)
	}
	if w.cfg.BoundaryR != s.BoundaryR {
		return fmt.Errorf("snapshot boundary_r mismatch: cfg=%d snap=%d", w.cfg.BoundaryR, s.BoundaryR)
	}
	return nil
}

func (w *World) applySnapshotConfig(s snapshot.SnapshotV1) {
	if s.BiomeRegionSize > 0 {
		w.cfg.BiomeRegionSize = s.BiomeRegionSize
	}
	if s.SpawnClearRadius > 0 {
		w.cfg.SpawnClearRadius = s.SpawnClearRadius
	}
	if s.OreClusterProbScalePermille > 0 {
		w.cfg.OreClusterProbScalePermille = s.OreClusterProbScalePermille
	}
	if s.TerrainClusterProbScalePermille > 0 {
		w.cfg.TerrainClusterProbScalePermille = s.TerrainClusterProbScalePermille
	}
	if s.SprinkleStonePermille > 0 {
		w.cfg.SprinkleStonePermille = s.SprinkleStonePermille
	}
	if s.SprinkleDirtPermille > 0 {
		w.cfg.SprinkleDirtPermille = s.SprinkleDirtPermille
	}
	if s.SprinkleLogPermille > 0 {
		w.cfg.SprinkleLogPermille = s.SprinkleLogPermille
	}

	if s.StarterItems != nil {
		w.cfg.StarterItems = map[string]int{}
		for item, n := range s.StarterItems {
			if strings.TrimSpace(item) == "" || n <= 0 {
				continue
			}
			w.cfg.StarterItems[item] = n
		}
	}

	if s.SnapshotEveryTicks > 0 {
		w.cfg.SnapshotEveryTicks = s.SnapshotEveryTicks
	}
	if s.DirectorEveryTicks > 0 {
		w.cfg.DirectorEveryTicks = s.DirectorEveryTicks
	}
	if s.SeasonLengthTicks > 0 {
		w.cfg.SeasonLengthTicks = s.SeasonLengthTicks
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
		w.cfg.RateLimits = RateLimitConfig{
			SayWindowTicks:        s.RateLimits.SayWindowTicks,
			SayMax:                s.RateLimits.SayMax,
			MarketSayWindowTicks:  s.RateLimits.MarketSayWindowTicks,
			MarketSayMax:          s.RateLimits.MarketSayMax,
			WhisperWindowTicks:    s.RateLimits.WhisperWindowTicks,
			WhisperMax:            s.RateLimits.WhisperMax,
			OfferTradeWindowTicks: s.RateLimits.OfferTradeWindowTicks,
			OfferTradeMax:         s.RateLimits.OfferTradeMax,
			PostBoardWindowTicks:  s.RateLimits.PostBoardWindowTicks,
			PostBoardMax:          s.RateLimits.PostBoardMax,
		}
	}
	if s.LawNoticeTicks > 0 {
		w.cfg.LawNoticeTicks = s.LawNoticeTicks
	}
	if s.LawVoteTicks > 0 {
		w.cfg.LawVoteTicks = s.LawVoteTicks
	}
	if s.BlueprintAutoPullRange > 0 {
		w.cfg.BlueprintAutoPullRange = s.BlueprintAutoPullRange
	}
	if s.BlueprintBlocksPerTick > 0 {
		w.cfg.BlueprintBlocksPerTick = s.BlueprintBlocksPerTick
	}
	if s.AccessPassCoreRadius > 0 {
		w.cfg.AccessPassCoreRadius = s.AccessPassCoreRadius
	}
	if len(s.MaintenanceCost) > 0 {
		w.cfg.MaintenanceCost = map[string]int{}
		for item, n := range s.MaintenanceCost {
			if item != "" && n > 0 {
				w.cfg.MaintenanceCost[item] = n
			}
		}
		if len(w.cfg.MaintenanceCost) == 0 {
			w.cfg.MaintenanceCost = nil
		}
	}
	if s.FunDecayWindowTicks > 0 {
		w.cfg.FunDecayWindowTicks = s.FunDecayWindowTicks
	}
	if s.FunDecayBase > 0 {
		w.cfg.FunDecayBase = s.FunDecayBase
	}
	if s.StructureSurvivalTicks > 0 {
		w.cfg.StructureSurvivalTicks = s.StructureSurvivalTicks
	}
	w.cfg.applyDefaults()
}

func (w *World) applySnapshotRuntimeState(s snapshot.SnapshotV1) {
	w.weather = s.Weather
	w.weatherUntilTick = s.WeatherUntilTick
	w.activeEventID = s.ActiveEventID
	w.activeEventStart = s.ActiveEventStart
	w.activeEventEnds = s.ActiveEventEnds
	w.activeEventCenter = Vec3i{
		X: s.ActiveEventCenter[0],
		Y: s.ActiveEventCenter[1],
		Z: s.ActiveEventCenter[2],
	}
	w.activeEventRadius = s.ActiveEventRadius
}
