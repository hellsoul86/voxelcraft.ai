package director

type Metrics struct {
	Trade       float64
	Conflict    float64
	Exploration float64
	Inequality  float64
}

func ApplyFeedback(weights map[string]float64, m Metrics) {
	if weights == nil {
		return
	}
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
}

func ItemUnitValue(item string) float64 {
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
		return 0.5
	}
}
