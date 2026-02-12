package maintenance

func EffectiveCost(cost map[string]int) map[string]int {
	if len(cost) > 0 {
		return cost
	}
	return map[string]int{"IRON_INGOT": 1, "COAL": 1}
}

func NextDue(nowTick, dueTick, dayTicks uint64) uint64 {
	if dayTicks == 0 {
		return dueTick
	}
	if dueTick == 0 {
		return nowTick + dayTicks
	}
	return dueTick + dayTicks
}

func NextStage(currentStage int, paid bool) int {
	if paid {
		return 0
	}
	if currentStage < 2 {
		return currentStage + 1
	}
	return currentStage
}
