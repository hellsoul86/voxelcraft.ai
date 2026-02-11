package contracts

func TradeCreditDelta() (trade int, social int) {
	return 6, 2
}

func BuildCreditDelta() int {
	return 6
}

func ClampReputation(v int) int {
	if v < 0 {
		return 0
	}
	if v > 1000 {
		return 1000
	}
	return v
}

func DepositMultiplier(repTrade int) int {
	switch {
	case repTrade >= 500:
		return 1
	case repTrade >= 300:
		return 2
	default:
		return 3
	}
}

func LawPenaltyDelta(reason string) (trade int, law int) {
	switch reason {
	case "CONTRACT_TIMEOUT":
		return -12, -8
	default:
		return 0, -4
	}
}

func HasAvailable(req map[string]int, available func(item string) int) bool {
	if len(req) == 0 {
		return true
	}
	for item, n := range req {
		if available(item) < n {
			return false
		}
	}
	return true
}
