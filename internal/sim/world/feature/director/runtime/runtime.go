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

func SeasonLengthTicks(resetEveryTicks int, seasonLengthTicks int) uint64 {
	seasonLen := uint64(resetEveryTicks)
	if seasonLen == 0 {
		seasonLen = uint64(seasonLengthTicks)
	}
	return seasonLen
}

func SeasonIndex(nowTick uint64, resetEveryTicks int, seasonLengthTicks int) int {
	seasonLen := SeasonLengthTicks(resetEveryTicks, seasonLengthTicks)
	if seasonLen == 0 {
		return 1
	}
	return int(nowTick/seasonLen) + 1
}

func SeasonDay(nowTick uint64, dayTicks int, resetEveryTicks int, seasonLengthTicks int) int {
	dt := uint64(dayTicks)
	if dt == 0 {
		return 1
	}
	seasonLen := SeasonLengthTicks(resetEveryTicks, seasonLengthTicks)
	seasonDays := seasonLen / dt
	if seasonDays == 0 {
		seasonDays = 1
	}
	return int((nowTick/dt)%seasonDays) + 1
}

func ShouldWorldResetNotice(nowTick uint64, resetEveryTicks int, resetNoticeTicks int) (bool, uint64) {
	cycle := uint64(resetEveryTicks)
	notice := uint64(resetNoticeTicks)
	if cycle == 0 || notice == 0 || notice >= cycle || nowTick == 0 {
		return false, 0
	}
	if nowTick%cycle != cycle-notice {
		return false, 0
	}
	return true, nowTick + notice
}
