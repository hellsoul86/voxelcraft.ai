package world

import (
	featuredirector "voxelcraft.ai/internal/sim/world/feature/director"
	"voxelcraft.ai/internal/sim/world/logic/directorcenter"
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

	tradePerAgent := float64(sum.Trades) / float64(agents)
	trade := directorcenter.Min01(tradePerAgent / 5.0)

	deniedPerTickPerAgent := float64(sum.Denied) / float64(uint64(agents)*windowTicks)
	conflict := directorcenter.Min01(deniedPerTickPerAgent * 100.0) // ~0.1 when ~1 denied / 1000 ticks / agent

	chunksPerAgent := float64(sum.ChunksDiscovered) / float64(agents)
	exploration := directorcenter.Min01(chunksPerAgent / 20.0)

	infraPerAgent := float64(sum.BlueprintsComplete) / float64(agents)
	publicInfra := directorcenter.Min01(infraPerAgent / 5.0)

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
	values := make([]float64, 0, len(agents))
	for _, a := range agents {
		if a == nil {
			continue
		}
		v := wealthValue(a.Inventory)
		values = append(values, v)
	}
	return directorcenter.Gini(values)
}

func wealthValue(inv map[string]int) float64 {
	return directorcenter.MapValue(inv, itemUnitValue)
}

func itemUnitValue(item string) float64 {
	return featuredirector.ItemUnitValue(item)
}
