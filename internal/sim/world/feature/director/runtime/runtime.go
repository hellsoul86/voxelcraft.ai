package runtime

type State struct {
	ActiveEventID   string
	ActiveEventEnds uint64
	Weather         string
	WeatherUntil    uint64
}

type ExpireResult struct {
	ActiveCleared  bool
	WeatherCleared bool
}

func Expire(in State, nowTick uint64) (State, ExpireResult) {
	out := in
	res := ExpireResult{}
	if out.ActiveEventEnds != 0 && nowTick >= out.ActiveEventEnds {
		out.ActiveEventID = ""
		out.ActiveEventEnds = 0
		res.ActiveCleared = true
	}
	if out.WeatherUntil != 0 && nowTick >= out.WeatherUntil {
		out.Weather = "CLEAR"
		out.WeatherUntil = 0
		res.WeatherCleared = true
	}
	return out, res
}

func ScriptedEvent(dayInSeason int) string {
	schedule := []string{
		"MARKET_WEEK",
		"CRYSTAL_RIFT",
		"BUILDER_EXPO",
		"FLOOD_WARNING",
		"RUINS_GATE",
		"BANDIT_CAMP",
		"CIVIC_VOTE",
	}
	if dayInSeason < 1 || dayInSeason > len(schedule) {
		return ""
	}
	return schedule[dayInSeason-1]
}

func ShouldEvaluate(nowTick uint64, every uint64) bool {
	if every == 0 {
		every = 3000
	}
	if nowTick == 0 {
		return false
	}
	return nowTick%every == 0
}
