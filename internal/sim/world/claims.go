package world

import "fmt"

type ClaimFlags struct {
	AllowBuild  bool
	AllowBreak  bool
	AllowDamage bool
	AllowTrade  bool
}

type LandClaim struct {
	LandID  string
	Owner   string // agent id (MVP)
	Anchor  Vec3i
	Radius  int // square radius in blocks
	Flags   ClaimFlags
	Members map[string]bool // agent ids

	// MVP configurable parameters (via laws or direct settings).
	MarketTax     float64 // 0..1
	CurfewEnabled bool
	CurfewStart   float64 // 0..1 time_of_day
	CurfewEnd     float64 // 0..1 time_of_day

	// Law: fine for illegal break attempts.
	FineBreakEnabled  bool
	FineBreakItem     string
	FineBreakPerBlock int

	// Law: access pass for the claim's "core" area.
	AccessPassEnabled bool
	AccessTicketItem  string
	AccessTicketCost  int

	// Maintenance: stage 0=ok, 1=late (no expansion), 2=unprotected.
	MaintenanceDueTick uint64
	MaintenanceStage   int
}

func (c *LandClaim) Contains(pos Vec3i) bool {
	dx := pos.X - c.Anchor.X
	if dx < 0 {
		dx = -dx
	}
	dz := pos.Z - c.Anchor.Z
	if dz < 0 {
		dz = -dz
	}
	return dx <= c.Radius && dz <= c.Radius
}

func (w *World) landCoreRadius(c *LandClaim) int {
	// MVP default: fixed core radius (configurable), capped by land radius.
	if c == nil {
		return 0
	}
	r := w.cfg.AccessPassCoreRadius
	if r <= 0 {
		r = 16
	}
	if c.Radius < r {
		r = c.Radius
	}
	if r < 0 {
		r = 0
	}
	return r
}

func (w *World) landCoreContains(c *LandClaim, pos Vec3i) bool {
	r := w.landCoreRadius(c)
	if r <= 0 || c == nil {
		return false
	}
	dx := pos.X - c.Anchor.X
	if dx < 0 {
		dx = -dx
	}
	dz := pos.Z - c.Anchor.Z
	if dz < 0 {
		dz = -dz
	}
	return dx <= r && dz <= r
}

func (w *World) landAt(pos Vec3i) *LandClaim {
	for _, c := range w.claims {
		if c.Contains(pos) {
			return c
		}
	}
	return nil
}

func (w *World) permissionsFor(agentID string, pos Vec3i) (land *LandClaim, perms map[string]bool) {
	land = w.landAt(pos)
	if land == nil {
		return nil, map[string]bool{
			"can_build":  true,
			"can_break":  true,
			"can_damage": false,
			"can_trade":  true,
		}
	}
	// Maintenance downgrade: if land is unprotected, treat visitor permissions as "wild".
	if land.MaintenanceStage >= 2 && !w.isLandMember(agentID, land) {
		return land, map[string]bool{
			"can_build":  true,
			"can_break":  true,
			"can_damage": false,
			"can_trade":  true,
		}
	}
	if w.isLandMember(agentID, land) {
		return land, map[string]bool{
			"can_build":  true,
			"can_break":  true,
			"can_damage": land.Flags.AllowDamage,
			"can_trade":  true,
		}
	}
	return land, map[string]bool{
		"can_build":  land.Flags.AllowBuild,
		"can_break":  land.Flags.AllowBreak,
		"can_damage": land.Flags.AllowDamage,
		"can_trade":  land.Flags.AllowTrade,
	}
}

func (w *World) canBuildAt(agentID string, pos Vec3i, nowTick uint64) bool {
	land, perms := w.permissionsFor(agentID, pos)
	if !perms["can_build"] {
		return false
	}
	if land == nil || !land.CurfewEnabled {
		return true
	}
	t := w.timeOfDay(nowTick)
	if inWindow(t, land.CurfewStart, land.CurfewEnd) {
		return false
	}
	return true
}

func (w *World) canBreakAt(agentID string, pos Vec3i, nowTick uint64) bool {
	land, perms := w.permissionsFor(agentID, pos)
	if !perms["can_break"] {
		return false
	}
	if land == nil || !land.CurfewEnabled {
		return true
	}
	t := w.timeOfDay(nowTick)
	if inWindow(t, land.CurfewStart, land.CurfewEnd) {
		return false
	}
	return true
}

func (w *World) timeOfDay(nowTick uint64) float64 {
	if w.cfg.DayTicks <= 0 {
		return 0
	}
	day := uint64(w.cfg.DayTicks)
	return float64(nowTick%day) / float64(day)
}

func (w *World) newLandID(owner string) string {
	n := w.nextLandNum.Add(1)
	return fmt.Sprintf("LAND_%s_%03d", owner, n)
}

func (w *World) removeClaimByAnchor(nowTick uint64, actor string, anchor Vec3i, reason string) {
	if len(w.claims) == 0 {
		return
	}
	// Deterministic: if multiple claims share an anchor (shouldn't happen), remove the smallest land_id.
	landID := ""
	for id, c := range w.claims {
		if c == nil || c.Anchor != anchor {
			continue
		}
		if landID == "" || id < landID {
			landID = id
		}
	}
	if landID == "" {
		return
	}
	delete(w.claims, landID)

	// Remove laws bound to this land (safety).
	if len(w.laws) > 0 {
		for id, l := range w.laws {
			if l == nil {
				continue
			}
			if l.LandID == landID {
				delete(w.laws, id)
			}
		}
	}

	w.auditEvent(nowTick, actor, "CLAIM_REMOVE", anchor, reason, map[string]any{
		"land_id": landID,
	})
}

func inWindow(t, start, end float64) bool {
	if start <= end {
		return t >= start && t <= end
	}
	// Wrap-around window.
	return t >= start || t <= end
}
