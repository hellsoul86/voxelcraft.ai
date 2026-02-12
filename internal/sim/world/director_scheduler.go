package world

import (
	"sort"

	feedbackpkg "voxelcraft.ai/internal/sim/world/feature/director/feedback"
	metricspkg "voxelcraft.ai/internal/sim/world/feature/director/metrics"
	runtimepkg "voxelcraft.ai/internal/sim/world/feature/director/runtime"
	"voxelcraft.ai/internal/sim/world/logic/directorcenter"
)

type directorMetrics struct {
	Trade       float64 // 0..1
	Conflict    float64 // 0..1
	Exploration float64 // 0..1
	Inequality  float64 // 0..1 (Gini)
	PublicInfra float64 // 0..1
}

func (w *World) systemDirector(nowTick uint64) {
	next, _ := runtimepkg.Expire(runtimepkg.State{
		ActiveEventID:   w.activeEventID,
		ActiveEventEnds: w.activeEventEnds,
		Weather:         w.weather,
		WeatherUntil:    w.weatherUntilTick,
	}, nowTick)
	if next.ActiveEventID == "" && w.activeEventID != "" {
		w.activeEventStart = 0
		w.activeEventCenter = Vec3i{}
		w.activeEventRadius = 0
	}
	w.activeEventID = next.ActiveEventID
	w.activeEventEnds = next.ActiveEventEnds
	w.weather = next.Weather
	w.weatherUntilTick = next.WeatherUntil

	// If an event is still active, don't schedule a new one.
	if w.activeEventID != "" {
		return
	}

	// First-week scripted cadence at the start of each in-game day.
	if w.cfg.DayTicks > 0 && nowTick%uint64(w.cfg.DayTicks) == 0 {
		dayInSeason := w.seasonDay(nowTick)
		if ev := runtimepkg.ScriptedEvent(dayInSeason); ev != "" {
			w.startEvent(nowTick, ev)
			return
		}
	}

	// After week 1, evaluate every N ticks (default 3000 ~= 10 minutes at 5Hz).
	every := uint64(w.cfg.DirectorEveryTicks)
	if !runtimepkg.ShouldEvaluate(nowTick, every) {
		return
	}

	m := w.computeDirectorMetrics(nowTick)
	weights := w.baseEventWeights()

	feedbackpkg.ApplyFeedback(weights, feedbackpkg.Metrics{
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

func (w *World) computeDirectorMetrics(nowTick uint64) directorMetrics {
	agents := len(w.agents)
	if agents <= 0 {
		return directorMetrics{}
	}

	sum := StatsBucket{}
	windowTicks := uint64(0)
	if w.stats != nil {
		sum = w.stats.Summarize(nowTick)
		windowTicks = w.stats.WindowTicks()
	}
	if windowTicks == 0 {
		windowTicks = 72000
	}

	wealth := make([]float64, 0, len(w.agents))
	for _, a := range w.sortedAgents() {
		if a == nil {
			continue
		}
		wealth = append(wealth, metricspkg.InventoryValue(a.Inventory))
	}
	m := metricspkg.ComputeMetrics(metricspkg.EvalInput{
		Agents:      agents,
		WindowTicks: windowTicks,
		Trades:      sum.Trades,
		Denied:      sum.Denied,
		Chunks:      sum.ChunksDiscovered,
		Blueprints:  sum.BlueprintsComplete,
		Wealth:      wealth,
	})

	return directorMetrics{
		Trade:       m.Trade,
		Conflict:    m.Conflict,
		Exploration: m.Exploration,
		Inequality:  m.Inequality,
		PublicInfra: m.PublicInfra,
	}
}
