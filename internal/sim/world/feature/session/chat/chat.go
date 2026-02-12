package chat

import "strings"

type RateLimits struct {
	SayWindowTicks       uint64
	SayMax               int
	MarketSayWindowTicks uint64
	MarketSayMax         int
}

type RateLimitSpec struct {
	Kind       string
	Window     uint64
	Max        int
	RateErrMsg string
}

func NormalizeChatChannel(raw string) (string, bool) {
	ch := strings.ToUpper(strings.TrimSpace(raw))
	if ch == "" {
		ch = "LOCAL"
	}
	switch ch {
	case "LOCAL", "CITY", "MARKET":
		return ch, true
	default:
		return "", false
	}
}

func LimitSpec(ch string, limits RateLimits) RateLimitSpec {
	spec := RateLimitSpec{
		Kind:       "SAY",
		Window:     limits.SayWindowTicks,
		Max:        limits.SayMax,
		RateErrMsg: "too many SAY",
	}
	if ch == "MARKET" {
		spec.Kind = "SAY_MARKET"
		spec.Window = limits.MarketSayWindowTicks
		spec.Max = limits.MarketSayMax
		spec.RateErrMsg = "too many SAY (MARKET)"
	}
	return spec
}
