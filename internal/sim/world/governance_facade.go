package world

import (
	"fmt"

	"voxelcraft.ai/internal/protocol"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	claimspkg "voxelcraft.ai/internal/sim/world/feature/governance/claims"
	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
	lawsruntimepkg "voxelcraft.ai/internal/sim/world/feature/governance/laws/runtime"
	permissionspkg "voxelcraft.ai/internal/sim/world/feature/governance/permissions"
	governanceruntimepkg "voxelcraft.ai/internal/sim/world/feature/governance/runtime"
)

// --- Claims / Permissions ---

func (w *World) landCoreRadius(c *LandClaim) int {
	if c == nil {
		return 0
	}
	return claimspkg.CoreRadius(c.Radius, w.cfg.AccessPassCoreRadius)
}

func (w *World) landCoreContains(c *LandClaim, pos Vec3i) bool {
	r := w.landCoreRadius(c)
	if c == nil {
		return false
	}
	return claimspkg.CoreContains(c.Anchor.X, c.Anchor.Z, pos.X, pos.Z, r)
}

func (w *World) landAt(pos Vec3i) *LandClaim {
	return governanceruntimepkg.LandAt(w.claims, pos)
}

func (w *World) permissionsFor(agentID string, pos Vec3i) (land *LandClaim, perms map[string]bool) {
	land = w.landAt(pos)
	if land == nil {
		p2 := permissionspkg.WildPermissions()
		return nil, map[string]bool{
			"can_build":  p2.CanBuild,
			"can_break":  p2.CanBreak,
			"can_damage": p2.CanDamage,
			"can_trade":  p2.CanTrade,
		}
	}
	fp := permissionspkg.ForLand(w.isLandMember(agentID, land), land.MaintenanceStage, claimspkg.Flags{
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
	return claimspkg.CanActionWithCurfew(
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
	return claimspkg.CanActionWithCurfew(
		perms["can_break"],
		land.CurfewEnabled,
		w.timeOfDay(nowTick),
		land.CurfewStart,
		land.CurfewEnd,
	)
}

func (w *World) timeOfDay(nowTick uint64) float64 {
	return claimspkg.TimeOfDay(nowTick, w.cfg.DayTicks)
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

// --- Organizations ---

func (w *World) orgTreasury(o *Organization) map[string]int {
	if o == nil {
		return nil
	}
	return o.TreasuryFor(w.cfg.ID)
}

func (w *World) newOrgID() string {
	n := w.nextOrgNum.Add(1)
	return fmt.Sprintf("ORG%06d", n)
}

func (w *World) orgByID(id string) *Organization {
	if id == "" {
		return nil
	}
	return w.orgs[id]
}

func (w *World) isOrgMember(agentID, orgID string) bool {
	return governanceruntimepkg.IsOrgMember(w.orgs, agentID, orgID)
}

func (w *World) isOrgAdmin(agentID, orgID string) bool {
	return governanceruntimepkg.IsOrgAdmin(w.orgs, agentID, orgID)
}

func (w *World) isLandAdmin(agentID string, land *LandClaim) bool {
	return governanceruntimepkg.IsLandAdmin(w.orgs, agentID, land)
}

func (w *World) isLandMember(agentID string, land *LandClaim) bool {
	return governanceruntimepkg.IsLandMember(w.orgs, agentID, land)
}

// --- Laws ---

type LawStatus = lawspkg.Status

const (
	LawNotice   LawStatus = lawspkg.StatusNotice
	LawVoting   LawStatus = lawspkg.StatusVoting
	LawActive   LawStatus = lawspkg.StatusActive
	LawRejected LawStatus = lawspkg.StatusRejected
)

type Law = lawspkg.Law

func (w *World) newLawID() string {
	n := w.nextLawNum.Add(1)
	return fmt.Sprintf("LAW%06d", n)
}

func (w *World) tickLaws(nowTick uint64) {
	lawsruntimepkg.TickLaws(nowTick, w.laws, lawsruntimepkg.TickLawsHooks{
		OnEnterVoting: func(law *lawspkg.Law) {
			w.broadcastLawEvent(nowTick, "VOTING", law, "")
		},
		OnActivate: func(law *lawspkg.Law, _ int, _ int) error {
			land := w.claims[law.LandID]
			if land == nil {
				return fmt.Errorf("land not found")
			}
			return lawsruntimepkg.ApplyTemplateToLand(law, land)
		},
		OnActivated: func(law *lawspkg.Law, yes int, no int) {
			if proposer := w.agents[law.ProposedBy]; proposer != nil {
				w.funOnLawActive(proposer, nowTick)
			}
			if land := w.claims[law.LandID]; land != nil {
				w.auditEvent(nowTick, "WORLD", "LAW_ACTIVE", land.Anchor, "VOTE_PASSED", map[string]any{
					"law_id":      law.LawID,
					"land_id":     law.LandID,
					"template_id": law.TemplateID,
					"title":       law.Title,
					"yes":         yes,
					"no":          no,
					"params":      law.Params,
				})
			}
			w.broadcastLawEvent(nowTick, "ACTIVE", law, "")
		},
		OnRejected: func(law *lawspkg.Law, yes int, no int, reason string, cause error) {
			if land := w.claims[law.LandID]; land != nil {
				details := map[string]any{
					"law_id":      law.LawID,
					"land_id":     law.LandID,
					"template_id": law.TemplateID,
					"title":       law.Title,
					"yes":         yes,
					"no":          no,
				}
				if cause != nil {
					details["message"] = cause.Error()
				}
				auditReason := "VOTE_FAILED"
				if cause != nil {
					auditReason = "ACTIVATE_FAILED"
				}
				w.auditEvent(nowTick, "WORLD", "LAW_REJECTED", land.Anchor, auditReason, details)
			}
			msg := reason
			if cause != nil {
				msg = cause.Error()
			}
			w.broadcastLawEvent(nowTick, "REJECTED", law, msg)
		},
	})
}

func (w *World) broadcastLawEvent(nowTick uint64, kind string, law *Law, message string) {
	base := protocol.Event{
		"t":           nowTick,
		"type":        "LAW",
		"kind":        kind,
		"law_id":      law.LawID,
		"land_id":     law.LandID,
		"template_id": law.TemplateID,
		"title":       law.Title,
		"status":      string(law.Status),
	}
	if message != "" {
		base["message"] = message
	}
	if land := w.claims[law.LandID]; land != nil {
		if land.ClaimType != "" {
			base["claim_type"] = land.ClaimType
		}
	}
	for _, a := range w.agents {
		a.AddEvent(base)
	}
}

// --- Claim Maintenance ---

func (w *World) tickClaimsMaintenance(nowTick uint64) {
	governanceruntimepkg.TickClaimsMaintenance(governanceruntimepkg.TickClaimsMaintenanceInput{
		NowTick:  nowTick,
		DayTicks: w.cfg.DayTicks,
		Claims:   w.claims,
	}, governanceruntimepkg.TickClaimsMaintenanceHooks{
		Pay: w.payMaintenance,
		Notify: func(c *LandClaim, status string) {
			if c == nil {
				return
			}
			if owner := w.agents[c.Owner]; owner != nil {
				owner.AddEvent(protocol.Event{
					"t":             nowTick,
					"type":          "MAINTENANCE",
					"land_id":       c.LandID,
					"status":        status,
					"stage":         c.MaintenanceStage,
					"next_due_tick": c.MaintenanceDueTick,
				})
			}
		},
	})
}

func (w *World) payMaintenance(c *LandClaim) bool {
	return governanceruntimepkg.PayMaintenance(governanceruntimepkg.PayMaintenanceInput{
		Claim:       c,
		DefaultCost: w.cfg.MaintenanceCost,
		GetOrg:      w.orgByID,
		OrgTreasury: w.orgTreasury,
		GetAgent: func(owner string) *Agent {
			return w.agents[owner]
		},
		HasItems:    inventorypkg.HasItems,
		DeductItems: inventorypkg.DeductItems,
	})
}
