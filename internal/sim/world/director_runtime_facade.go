package world

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
	eventspkg "voxelcraft.ai/internal/sim/world/feature/director/events"
	feedbackpkg "voxelcraft.ai/internal/sim/world/feature/director/feedback"
	metricspkg "voxelcraft.ai/internal/sim/world/feature/director/metrics"
	runtimepkg "voxelcraft.ai/internal/sim/world/feature/director/runtime"
	"voxelcraft.ai/internal/sim/world/logic/directorcenter"
	genpkg "voxelcraft.ai/internal/sim/world/terrain/gen"
)

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
	ev := directorcenter.SampleWeighted(weights, genpkg.Hash2(w.cfg.Seed, int(nowTick), 1337))
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

func (w *World) computeDirectorMetrics(nowTick uint64) metricspkg.EvalMetrics {
	agents := len(w.agents)
	if agents <= 0 {
		return metricspkg.EvalMetrics{}
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

	return m
}

func (w *World) startEvent(nowTick uint64, eventID string) {
	tpl, ok := w.catalogs.Events.ByID[eventID]
	if !ok {
		return
	}
	duration := uint64(w.cfg.DayTicks)
	if duration == 0 {
		duration = 6000
	}
	if v, ok := tpl.Params["duration_ticks"]; ok {
		if f, ok := v.(float64); ok && f > 0 {
			duration = uint64(f)
		}
	}

	w.activeEventID = eventID
	w.activeEventStart = nowTick
	w.activeEventEnds = nowTick + duration

	// Instantiate event effects (e.g. spawn a resource node).
	w.instantiateEvent(nowTick, eventID)

	// Optional weather overrides.
	switch eventID {
	case "STORM_FRONT":
		w.weather = "STORM"
		w.weatherUntilTick = w.activeEventEnds
	case "COLD_SNAP":
		w.weather = "COLD"
		w.weatherUntilTick = w.activeEventEnds
	}

	for _, a := range w.agents {
		ev := protocol.Event{
			"t":         nowTick,
			"type":      "WORLD_EVENT",
			"event_id":  eventID,
			"title":     tpl.Title,
			"summary":   tpl.Description,
			"ends_tick": w.activeEventEnds,
		}
		if w.activeEventRadius > 0 {
			ev["center"] = w.activeEventCenter.ToArray()
			ev["radius"] = w.activeEventRadius
		}
		a.AddEvent(ev)
	}
}

func (w *World) enqueueActiveEventForAgent(nowTick uint64, a *Agent) {
	if a == nil || w.activeEventID == "" || w.activeEventEnds == 0 || nowTick >= w.activeEventEnds {
		return
	}
	tpl, ok := w.catalogs.Events.ByID[w.activeEventID]
	if !ok {
		return
	}
	ev := protocol.Event{
		"t":         nowTick,
		"type":      "WORLD_EVENT",
		"event_id":  w.activeEventID,
		"title":     tpl.Title,
		"summary":   tpl.Description,
		"ends_tick": w.activeEventEnds,
	}
	if w.activeEventRadius > 0 {
		ev["center"] = w.activeEventCenter.ToArray()
		ev["radius"] = w.activeEventRadius
	}
	a.AddEvent(ev)
}

func (w *World) instantiateEvent(nowTick uint64, eventID string) {
	// Default: no location.
	w.activeEventCenter = Vec3i{}
	w.activeEventRadius = 0

	params := map[string]any{}
	if tpl, ok := w.catalogs.Events.ByID[eventID]; ok && tpl.Params != nil {
		params = tpl.Params
	}
	plan := eventspkg.BuildInstantiatePlan(eventID, params)
	if !plan.NeedsCenter {
		return
	}
	center := w.pickEventCenter(nowTick, eventID)
	w.activeEventCenter = center
	w.activeEventRadius = plan.Radius

	didNotice := false
	switch plan.Spawn {
	case eventspkg.SpawnCrystalRift:
		w.spawnCrystalRift(nowTick, center)
	case eventspkg.SpawnDeepVein:
		w.spawnDeepVein(nowTick, center)
	case eventspkg.SpawnRuinsGate:
		w.spawnRuinsGate(nowTick, center)
	case eventspkg.SpawnFloodWarning:
		w.spawnFloodWarning(nowTick, center)
	case eventspkg.SpawnBanditCamp:
		w.spawnBanditCamp(nowTick, center)
	case eventspkg.SpawnBlightZone:
		w.spawnBlightZone(nowTick, center)
	case eventspkg.SpawnNoticeBoard:
		didNotice = true
		w.spawnEventNoticeBoard(nowTick, center, eventID, plan.Headline, plan.Body)
	}
	if !didNotice && plan.Headline != "" {
		w.spawnEventNoticeBoard(nowTick, center, eventID, plan.Headline, plan.Body)
	}
}

func (w *World) pickEventCenter(nowTick uint64, eventID string) Vec3i {
	p := directorcenter.PickEventCenter(
		w.cfg.Seed,
		w.cfg.BoundaryR,
		nowTick,
		eventID,
		func(dp directorcenter.Pos) bool {
			return w.landAt(Vec3i{X: dp.X, Y: dp.Y, Z: dp.Z}) != nil
		},
	)
	return Vec3i{X: p.X, Y: p.Y, Z: p.Z}
}
