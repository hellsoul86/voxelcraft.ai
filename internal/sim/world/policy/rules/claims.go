package rules

type ClaimFlags struct {
	AllowBuild  bool
	AllowBreak  bool
	AllowDamage bool
	AllowTrade  bool
}

func DefaultClaimTypeForWorld(worldType string) string {
	switch worldType {
	case "OVERWORLD":
		return "HOMESTEAD"
	case "CITY_HUB":
		return "CITY_CORE"
	default:
		return "DEFAULT"
	}
}

func DefaultClaimFlags(claimType string) ClaimFlags {
	switch claimType {
	case "CITY_CORE":
		return ClaimFlags{AllowBuild: false, AllowBreak: false, AllowDamage: false, AllowTrade: true}
	case "HOMESTEAD":
		return ClaimFlags{AllowBuild: false, AllowBreak: false, AllowDamage: false, AllowTrade: false}
	default:
		return ClaimFlags{AllowBuild: false, AllowBreak: false, AllowDamage: false, AllowTrade: false}
	}
}

func InWindow(t, start, end float64) bool {
	if start <= end {
		return t >= start && t <= end
	}
	// Wrap-around window.
	return t >= start || t <= end
}

func CanActionWithCurfew(baseAllowed, curfewEnabled bool, now, start, end float64) bool {
	if !baseAllowed {
		return false
	}
	if !curfewEnabled {
		return true
	}
	return !InWindow(now, start, end)
}
