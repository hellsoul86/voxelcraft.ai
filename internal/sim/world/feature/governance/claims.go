package governance

import "voxelcraft.ai/internal/sim/world/policy/rules"

type ClaimFlags struct {
	AllowBuild  bool
	AllowBreak  bool
	AllowDamage bool
	AllowTrade  bool
}

func DefaultClaimTypeForWorld(worldType string) string {
	return rules.DefaultClaimTypeForWorld(worldType)
}

func DefaultClaimFlags(claimType string) ClaimFlags {
	f := rules.DefaultClaimFlags(claimType)
	return ClaimFlags{
		AllowBuild:  f.AllowBuild,
		AllowBreak:  f.AllowBreak,
		AllowDamage: f.AllowDamage,
		AllowTrade:  f.AllowTrade,
	}
}

func CanActionWithCurfew(baseAllowed bool, curfewEnabled bool, now, start, end float64) bool {
	return rules.CanActionWithCurfew(baseAllowed, curfewEnabled, now, start, end)
}

func InWindow(t, start, end float64) bool {
	return rules.InWindow(t, start, end)
}

func ParamFloat(params map[string]interface{}, key string) (float64, error) {
	return rules.ParamFloat(params, key)
}

func ParamInt(params map[string]interface{}, key string) (int, error) {
	return rules.ParamInt(params, key)
}

func ParamString(params map[string]interface{}, key string) (string, error) {
	return rules.ParamString(params, key)
}

func FloatToCanonString(f float64) string {
	return rules.FloatToCanonString(f)
}

func CoreRadius(landRadius int, configured int) int {
	r := configured
	if r <= 0 {
		r = 16
	}
	if landRadius < r {
		r = landRadius
	}
	if r < 0 {
		r = 0
	}
	return r
}

func CoreContains(anchorX, anchorZ, posX, posZ, coreRadius int) bool {
	if coreRadius <= 0 {
		return false
	}
	dx := posX - anchorX
	if dx < 0 {
		dx = -dx
	}
	dz := posZ - anchorZ
	if dz < 0 {
		dz = -dz
	}
	return dx <= coreRadius && dz <= coreRadius
}

func TimeOfDay(nowTick uint64, dayTicks int) float64 {
	if dayTicks <= 0 {
		return 0
	}
	day := uint64(dayTicks)
	return float64(nowTick%day) / float64(day)
}
