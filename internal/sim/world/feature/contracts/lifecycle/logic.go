package lifecycle

func BuildDeadline(nowTick uint64, explicitDeadline uint64, durationTicks int, dayTicks int) uint64 {
	if explicitDeadline != 0 {
		return explicitDeadline
	}
	dur := durationTicks
	if dur <= 0 {
		dur = dayTicks
	}
	if dur < 0 {
		dur = 0
	}
	return nowTick + uint64(dur)
}

func ValidatePostInput(kind string, requirements map[string]int, reward map[string]int) (ok bool, code string, msg string) {
	if len(reward) == 0 {
		return false, "E_BAD_REQUEST", "missing reward"
	}
	if kind != "BUILD" && len(requirements) == 0 {
		return false, "E_BAD_REQUEST", "missing requirements"
	}
	return true, "", ""
}

func ScaleDeposit(base map[string]int, multiplier int) map[string]int {
	if len(base) == 0 {
		return nil
	}
	if multiplier <= 1 {
		out := make(map[string]int, len(base))
		for item, n := range base {
			if item == "" || n <= 0 {
				continue
			}
			out[item] = n
		}
		return out
	}
	out := make(map[string]int, len(base))
	for item, n := range base {
		if item == "" || n <= 0 {
			continue
		}
		out[item] = n * multiplier
	}
	return out
}

func CanSubmit(kind string, requirementsOK bool, buildOK bool) bool {
	switch kind {
	case "GATHER", "DELIVER":
		return requirementsOK
	case "BUILD":
		return buildOK
	default:
		return false
	}
}

func NeedsRequirementsConsumption(kind string) bool {
	return kind == "GATHER" || kind == "DELIVER"
}
