package work

func MineProgress(workTicks int, blockName string, inventory map[string]int) float64 {
	family := MineToolFamilyForBlock(blockName)
	tier := BestToolTier(inventory, family)
	workNeeded, _ := MineParamsForTier(tier)
	return TimedProgress(workTicks, workNeeded)
}

func TimedProgress(workTicks int, totalTicks int) float64 {
	if totalTicks <= 0 {
		return 0
	}
	return clamp01(float64(workTicks) / float64(totalTicks))
}

func BlueprintProgress(buildIndex int, blockCount int) float64 {
	if blockCount <= 0 {
		return 0
	}
	return clamp01(float64(buildIndex) / float64(blockCount))
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
