package world

import (
	"math"
	"sort"

	"voxelcraft.ai/internal/protocol"
)

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
		w.activeEventEnds = 0
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
		day := int(nowTick/uint64(w.cfg.DayTicks)) + 1
		if day >= 1 && day <= len(schedule) {
			w.startEvent(nowTick, schedule[day-1])
			return
		}
	}

	// After week 1, evaluate every ~10 minutes.
	if nowTick == 0 || nowTick%3000 != 0 {
		return
	}

	m := w.computeDirectorMetrics(nowTick)
	weights := w.baseEventWeights()

	// Feedback rules (match the spec's intent; numbers are tunable).
	if m.Trade < 0.4 {
		weights["MARKET_WEEK"] += 0.25
		weights["BLUEPRINT_FAIR"] += 0.15
	}
	if m.Exploration < 0.3 {
		weights["CRYSTAL_RIFT"] += 0.20
		weights["RUINS_GATE"] += 0.20
	}
	if m.Conflict < 0.10 {
		weights["DEEP_VEIN"] += 0.15
		weights["BANDIT_CAMP"] += 0.10
	} else if m.Conflict > 0.25 {
		weights["CIVIC_VOTE"] += 0.25
		weights["MARKET_WEEK"] += 0.10
		weights["BUILDER_EXPO"] += 0.10
	}
	if m.Inequality > 0.50 {
		weights["CIVIC_VOTE"] += 0.20
		weights["FLOOD_WARNING"] += 0.10
	}

	// Sample deterministically using world seed + tick.
	ev := sampleWeighted(weights, hash2(w.cfg.Seed, int(nowTick), 1337))
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

func sampleWeighted(weights map[string]float64, roll uint64) string {
	if len(weights) == 0 {
		return ""
	}
	ids := make([]string, 0, len(weights))
	var total float64
	for id, w := range weights {
		if w > 0 {
			ids = append(ids, id)
			total += w
		}
	}
	if total <= 0 || len(ids) == 0 {
		return ""
	}
	sort.Strings(ids)

	// Deterministic pick in [0,total).
	r := float64(roll%1_000_000_000) / 1_000_000_000.0
	target := r * total

	var acc float64
	for _, id := range ids {
		acc += weights[id]
		if target <= acc {
			return id
		}
	}
	return ids[len(ids)-1]
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
	w.activeEventEnds = nowTick + duration

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
		a.AddEvent(protocol.Event{
			"t":         nowTick,
			"type":      "WORLD_EVENT",
			"event_id":  eventID,
			"title":     tpl.Title,
			"summary":   tpl.Description,
			"ends_tick": w.activeEventEnds,
		})
	}
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

	tradePerAgent := float64(sum.Trades) / float64(agents)
	trade := min1(tradePerAgent / 5.0)

	deniedPerTickPerAgent := float64(sum.Denied) / float64(uint64(agents)*windowTicks)
	conflict := min1(deniedPerTickPerAgent * 100.0) // ~0.1 when ~1 denied / 1000 ticks / agent

	chunksPerAgent := float64(sum.ChunksDiscovered) / float64(agents)
	exploration := min1(chunksPerAgent / 20.0)

	infraPerAgent := float64(sum.BlueprintsComplete) / float64(agents)
	publicInfra := min1(infraPerAgent / 5.0)

	inequality := giniWealth(w.sortedAgents())

	return directorMetrics{
		Trade:       trade,
		Conflict:    conflict,
		Exploration: exploration,
		Inequality:  inequality,
		PublicInfra: publicInfra,
	}
}

func giniWealth(agents []*Agent) float64 {
	if len(agents) <= 1 {
		return 0
	}
	values := make([]float64, 0, len(agents))
	var sum float64
	for _, a := range agents {
		if a == nil {
			continue
		}
		v := wealthValue(a.Inventory)
		values = append(values, v)
		sum += v
	}
	if len(values) <= 1 || sum <= 0 {
		return 0
	}
	sort.Float64s(values)

	// Gini coefficient: (2*sum_i i*x_i)/(n*sum x) - (n+1)/n, with i=1..n.
	n := float64(len(values))
	var weighted float64
	for i, x := range values {
		weighted += float64(i+1) * x
	}
	g := (2.0*weighted)/(n*sum) - (n+1.0)/n
	if g < 0 {
		return 0
	}
	if g > 1 {
		return 1
	}
	return g
}

func wealthValue(inv map[string]int) float64 {
	if len(inv) == 0 {
		return 0
	}
	var v float64
	for item, n := range inv {
		if n <= 0 {
			continue
		}
		v += float64(n) * itemUnitValue(item)
	}
	return v
}

func itemUnitValue(item string) float64 {
	switch item {
	case "CRYSTAL_SHARD":
		return 50
	case "IRON_INGOT":
		return 10
	case "COPPER_INGOT":
		return 6
	case "COAL":
		return 1
	case "PLANK":
		return 1
	default:
		// Default weight for unknown items to keep inequality defined.
		return 0.5
	}
}

func min1(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 || math.IsNaN(x) {
		return 1
	}
	return x
}
