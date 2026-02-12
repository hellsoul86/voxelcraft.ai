package metrics

import (
	"voxelcraft.ai/internal/sim/world/feature/director/feedback"
	"voxelcraft.ai/internal/sim/world/logic/directorcenter"
)

type EvalInput struct {
	Agents      int
	WindowTicks uint64
	Trades      int
	Denied      int
	Chunks      int
	Blueprints  int
	Wealth      []float64
}

type EvalMetrics struct {
	Trade       float64
	Conflict    float64
	Exploration float64
	Inequality  float64
	PublicInfra float64
}

func ComputeMetrics(in EvalInput) EvalMetrics {
	if in.Agents <= 0 {
		return EvalMetrics{}
	}
	windowTicks := in.WindowTicks
	if windowTicks == 0 {
		windowTicks = 72000
	}

	tradePerAgent := float64(in.Trades) / float64(in.Agents)
	trade := directorcenter.Min01(tradePerAgent / 5.0)

	deniedPerTickPerAgent := float64(in.Denied) / float64(uint64(in.Agents)*windowTicks)
	conflict := directorcenter.Min01(deniedPerTickPerAgent * 100.0)

	chunksPerAgent := float64(in.Chunks) / float64(in.Agents)
	exploration := directorcenter.Min01(chunksPerAgent / 20.0)

	infraPerAgent := float64(in.Blueprints) / float64(in.Agents)
	publicInfra := directorcenter.Min01(infraPerAgent / 5.0)

	inequality := directorcenter.Gini(in.Wealth)

	return EvalMetrics{
		Trade:       trade,
		Conflict:    conflict,
		Exploration: exploration,
		Inequality:  inequality,
		PublicInfra: publicInfra,
	}
}

func InventoryValue(inv map[string]int) float64 {
	return directorcenter.MapValue(inv, feedback.ItemUnitValue)
}
