package claims

import (
	"strings"

	"voxelcraft.ai/internal/sim/world/policy/rules"
)

type Flags struct {
	AllowBuild  bool
	AllowBreak  bool
	AllowDamage bool
	AllowTrade  bool
}

func DefaultClaimTypeForWorld(worldType string) string {
	return rules.DefaultClaimTypeForWorld(worldType)
}

func DefaultFlags(claimType string) Flags {
	f := rules.DefaultClaimFlags(claimType)
	return Flags{
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

func ValidateSetPermissionsInput(landID string, policy map[string]bool) (ok bool, code string, msg string) {
	if strings.TrimSpace(landID) == "" || policy == nil {
		return false, "E_BAD_REQUEST", "missing land_id/policy"
	}
	return true, "", ""
}

func ApplyPolicyFlags(flags Flags, policy map[string]bool) Flags {
	next := flags
	if v, ok := policy["allow_build"]; ok {
		next.AllowBuild = v
	}
	if v, ok := policy["allow_break"]; ok {
		next.AllowBreak = v
	}
	if v, ok := policy["allow_damage"]; ok {
		next.AllowDamage = v
	}
	if v, ok := policy["allow_trade"]; ok {
		next.AllowTrade = v
	}
	return next
}

func ValidateUpgradeInput(landID string, radius int) (ok bool, code string, msg string) {
	if strings.TrimSpace(landID) == "" || radius <= 0 {
		return false, "E_BAD_REQUEST", "missing land_id/radius"
	}
	return true, "", ""
}

func ValidateUpgradeRadius(current, target int) (ok bool, code string, msg string) {
	if target != 64 && target != 128 {
		return false, "E_BAD_REQUEST", "radius must be 64 or 128"
	}
	if target <= current {
		return false, "E_BAD_REQUEST", "radius must increase"
	}
	return true, "", ""
}

func UpgradeCost(current, target int) map[string]int {
	cost := map[string]int{}
	add := func(item string, n int) {
		if item == "" || n <= 0 {
			return
		}
		cost[item] += n
	}
	if current < 64 && target >= 64 {
		add("BATTERY", 1)
		add("CRYSTAL_SHARD", 2)
	}
	if current < 128 && target >= 128 {
		add("BATTERY", 2)
		add("CRYSTAL_SHARD", 4)
	}
	return cost
}

func ValidateMemberMutationInput(landID, memberID string) (ok bool, code string, msg string) {
	if strings.TrimSpace(landID) == "" || strings.TrimSpace(memberID) == "" {
		return false, "E_BAD_REQUEST", "missing land_id/member_id"
	}
	return true, "", ""
}

func NormalizeNewOwner(owner string) string {
	return strings.TrimSpace(owner)
}

func ValidateDeedInput(landID, newOwner string) (ok bool, code string, msg string) {
	if strings.TrimSpace(landID) == "" || strings.TrimSpace(newOwner) == "" {
		return false, "E_BAD_REQUEST", "missing land_id/new_owner"
	}
	return true, "", ""
}

type Zone struct {
	LandID  string
	AnchorX int
	AnchorZ int
	Radius  int
}

func UpgradeOverlaps(anchorX, anchorZ, targetRadius int, landID string, zones []Zone) bool {
	for _, z := range zones {
		if z.LandID == "" || z.LandID == landID {
			continue
		}
		dx := anchorX - z.AnchorX
		if dx < 0 {
			dx = -dx
		}
		dz := anchorZ - z.AnchorZ
		if dz < 0 {
			dz = -dz
		}
		if dx <= targetRadius+z.Radius && dz <= targetRadius+z.Radius {
			return true
		}
	}
	return false
}
