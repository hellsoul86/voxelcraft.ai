package world

import (
	"fmt"

	"voxelcraft.ai/internal/sim/world/feature/governance"
)

type ClaimFlags struct {
	AllowBuild  bool
	AllowBreak  bool
	AllowDamage bool
	AllowTrade  bool
}

const (
	ClaimTypeDefault   = "DEFAULT"
	ClaimTypeHomestead = "HOMESTEAD"
	ClaimTypeCityCore  = "CITY_CORE"
)

type LandClaim struct {
	LandID    string
	Owner     string // agent id (MVP)
	ClaimType string
	Anchor    Vec3i
	Radius    int // square radius in blocks
	Flags     ClaimFlags
	Members   map[string]bool // agent ids

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
	if c == nil {
		return 0
	}
	return governance.CoreRadius(c.Radius, w.cfg.AccessPassCoreRadius)
}

func (w *World) landCoreContains(c *LandClaim, pos Vec3i) bool {
	r := w.landCoreRadius(c)
	if c == nil {
		return false
	}
	return governance.CoreContains(c.Anchor.X, c.Anchor.Z, pos.X, pos.Z, r)
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
		p := governance.WildPermissions()
		return nil, map[string]bool{
			"can_build":  p.CanBuild,
			"can_break":  p.CanBreak,
			"can_damage": p.CanDamage,
			"can_trade":  p.CanTrade,
		}
	}
	fp := governance.PermissionsForLand(w.isLandMember(agentID, land), land.MaintenanceStage, governance.ClaimFlags{
		AllowBuild:  land.Flags.AllowBuild,
		AllowBreak:  land.Flags.AllowBreak,
		AllowDamage: land.Flags.AllowDamage,
		AllowTrade:  land.Flags.AllowTrade,
	})
	return land, map[string]bool{
		"can_build":  fp.CanBuild,
		"can_break":  fp.CanBreak,
		"can_damage": fp.CanDamage,
		"can_trade":  fp.CanTrade,
	}
}

func (w *World) canBuildAt(agentID string, pos Vec3i, nowTick uint64) bool {
	land, perms := w.permissionsFor(agentID, pos)
	if land == nil {
		return perms["can_build"]
	}
	return governance.CanActionWithCurfew(
		perms["can_build"],
		land.CurfewEnabled,
		w.timeOfDay(nowTick),
		land.CurfewStart,
		land.CurfewEnd,
	)
}

func (w *World) canBreakAt(agentID string, pos Vec3i, nowTick uint64) bool {
	land, perms := w.permissionsFor(agentID, pos)
	if land == nil {
		return perms["can_break"]
	}
	return governance.CanActionWithCurfew(
		perms["can_break"],
		land.CurfewEnabled,
		w.timeOfDay(nowTick),
		land.CurfewStart,
		land.CurfewEnd,
	)
}

func (w *World) timeOfDay(nowTick uint64) float64 {
	return governance.TimeOfDay(nowTick, w.cfg.DayTicks)
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
