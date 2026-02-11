package world

import "sort"
import "voxelcraft.ai/internal/sim/world/logic/directorcenter"
import featuredirector "voxelcraft.ai/internal/sim/world/feature/director"

type directorMetrics struct {
	Trade       float64 // 0..1
	Conflict    float64 // 0..1
	Exploration float64 // 0..1
	Inequality  float64 // 0..1 (Gini)
	PublicInfra float64 // 0..1
}

func (w *World) systemDirector(nowTick uint64) {
	// Expire active event.
	if w.activeEventEnds != 0 && nowTick >= w.activeEventEnds {
		w.activeEventID = ""
		w.activeEventStart = 0
		w.activeEventEnds = 0
		w.activeEventCenter = Vec3i{}
		w.activeEventRadius = 0
	}
	// Expire weather override.
	if w.weatherUntilTick != 0 && nowTick >= w.weatherUntilTick {
		w.weather = "CLEAR"
		w.weatherUntilTick = 0
	}

	// If an event is still active, don't schedule a new one.
	if w.activeEventID != "" {
		return
	}

	// First-week scripted cadence at the start of each in-game day.
	if w.cfg.DayTicks > 0 && nowTick%uint64(w.cfg.DayTicks) == 0 {
		schedule := []string{
			"MARKET_WEEK",
			"CRYSTAL_RIFT",
			"BUILDER_EXPO",
			"FLOOD_WARNING",
			"RUINS_GATE",
			"BANDIT_CAMP",
			"CIVIC_VOTE",
		}
		dayInSeason := w.seasonDay(nowTick)
		if dayInSeason >= 1 && dayInSeason <= len(schedule) {
			w.startEvent(nowTick, schedule[dayInSeason-1])
			return
		}
	}

	// After week 1, evaluate every N ticks (default 3000 ~= 10 minutes at 5Hz).
	every := uint64(w.cfg.DirectorEveryTicks)
	if every == 0 {
		every = 3000
	}
	if nowTick == 0 || nowTick%every != 0 {
		return
	}

	m := w.computeDirectorMetrics(nowTick)
	weights := w.baseEventWeights()

	featuredirector.ApplyFeedback(weights, featuredirector.Metrics{
		Trade:       m.Trade,
		Conflict:    m.Conflict,
		Exploration: m.Exploration,
		Inequality:  m.Inequality,
	})

	// Sample deterministically using world seed + tick.
	ev := directorcenter.SampleWeighted(weights, hash2(w.cfg.Seed, int(nowTick), 1337))
	if ev == "" {
		return
	}
	w.startEvent(nowTick, ev)
}

func (w *World) baseEventWeights() map[string]float64 {
	weights := map[string]float64{}
	ids := make([]string, 0, len(w.catalogs.Events.ByID))
	for id := range w.catalogs.Events.ByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		t := w.catalogs.Events.ByID[id]
		if t.BaseWeight <= 0 {
			continue
		}
		weights[id] = t.BaseWeight
	}
	// Avoid immediate repeats.
	if w.activeEventID != "" {
		weights[w.activeEventID] = 0
	}
	return weights
}
