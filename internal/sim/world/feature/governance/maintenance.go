package governance

func EffectiveMaintenanceCost(cost map[string]int) map[string]int {
	if len(cost) > 0 {
		return cost
	}
	return map[string]int{"IRON_INGOT": 1, "COAL": 1}
}

func NextMaintenanceDue(nowTick, dueTick, dayTicks uint64) uint64 {
	if dayTicks == 0 {
		return dueTick
	}
	if dueTick == 0 {
		return nowTick + dayTicks
	}
	return dueTick + dayTicks
}

func NextMaintenanceStage(currentStage int, paid bool) int {
	if paid {
		return 0
	}
	if currentStage < 2 {
		return currentStage + 1
	}
	return currentStage
}
