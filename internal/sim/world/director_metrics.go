package world

import (
	featuredirector "voxelcraft.ai/internal/sim/world/feature/director"
)

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
		wealth = append(wealth, featuredirector.InventoryValue(a.Inventory))
	}
	m := featuredirector.ComputeMetrics(featuredirector.EvalInput{
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
