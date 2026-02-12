package world

import (
	"fmt"
	"sort"
	"strings"

	"voxelcraft.ai/internal/protocol"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	claimspkg "voxelcraft.ai/internal/sim/world/feature/governance/claims"
	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
	lawsruntimepkg "voxelcraft.ai/internal/sim/world/feature/governance/laws/runtime"
	maintenancepkg "voxelcraft.ai/internal/sim/world/feature/governance/maintenance"
	permissionspkg "voxelcraft.ai/internal/sim/world/feature/governance/permissions"
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
	o := w.orgByID(orgID)
	if o == nil || o.Members == nil {
		return false
	}
	_, ok := o.Members[agentID]
	return ok
}

func (w *World) isOrgAdmin(agentID, orgID string) bool {
	o := w.orgByID(orgID)
	if o == nil || o.Members == nil {
		return false
	}
	role := o.Members[agentID]
	return role == OrgLeader || role == OrgOfficer
}

func (w *World) isLandAdmin(agentID string, land *LandClaim) bool {
	if land == nil {
		return false
	}
	if land.Owner == agentID {
		return true
	}
	// If an org owns the land, leaders/officers are admins.
	return w.isOrgAdmin(agentID, land.Owner)
}

func (w *World) isLandMember(agentID string, land *LandClaim) bool {
	if land == nil {
		return false
	}
	if land.Owner == agentID {
		return true
	}
	if land.Members != nil && land.Members[agentID] {
		return true
	}
	// If an org owns the land, any org member is treated as land member.
	return w.isOrgMember(agentID, land.Owner)
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
	if len(w.laws) == 0 {
		return
	}
	ids := make([]string, 0, len(w.laws))
	for id := range w.laws {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		law := w.laws[id]
		if law == nil {
			continue
		}
		tr := lawsruntimepkg.NextTransition(lawsruntimepkg.TransitionInput{
			Status:         string(law.Status),
			NowTick:        nowTick,
			NoticeEndsTick: law.NoticeEndsTick,
			VoteEndsTick:   law.VoteEndsTick,
		})
		if tr.ShouldTransition && tr.NextStatus == string(LawVoting) {
			law.Status = LawVoting
			w.broadcastLawEvent(nowTick, tr.EventKind, law, "")
			continue
		}
		if tr.ShouldTransition && law.Status == LawVoting {
			yes, no := lawspkg.CountVotes(law.Votes)
			if lawsruntimepkg.VotePassed(yes, no) {
				if err := w.activateLaw(nowTick, law); err != nil {
					law.Status = LawRejected
					if land := w.claims[law.LandID]; land != nil {
						w.auditEvent(nowTick, "WORLD", "LAW_REJECTED", land.Anchor, "ACTIVATE_FAILED", map[string]any{
							"law_id":      law.LawID,
							"land_id":     law.LandID,
							"template_id": law.TemplateID,
							"title":       law.Title,
							"yes":         yes,
							"no":          no,
							"message":     err.Error(),
						})
					}
					w.broadcastLawEvent(nowTick, "REJECTED", law, err.Error())
					continue
				}
				if proposer := w.agents[law.ProposedBy]; proposer != nil {
					w.funOnLawActive(proposer, nowTick)
				}
				law.Status = LawActive
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
			} else {
				law.Status = LawRejected
				if land := w.claims[law.LandID]; land != nil {
					w.auditEvent(nowTick, "WORLD", "LAW_REJECTED", land.Anchor, "VOTE_FAILED", map[string]any{
						"law_id":      law.LawID,
						"land_id":     law.LandID,
						"template_id": law.TemplateID,
						"title":       law.Title,
						"yes":         yes,
						"no":          no,
					})
				}
				w.broadcastLawEvent(nowTick, "REJECTED", law, "vote failed")
			}
		}
	}
}

func (w *World) activateLaw(_ uint64, law *Law) error {
	if law == nil {
		return fmt.Errorf("nil law")
	}
	land := w.claims[law.LandID]
	if land == nil {
		return fmt.Errorf("land not found")
	}
	out, err := lawspkg.ApplyLawTemplate(law.TemplateID, law.Params, lawspkg.LandState{
		MarketTax:         land.MarketTax,
		CurfewEnabled:     land.CurfewEnabled,
		CurfewStart:       land.CurfewStart,
		CurfewEnd:         land.CurfewEnd,
		FineBreakEnabled:  land.FineBreakEnabled,
		FineBreakItem:     land.FineBreakItem,
		FineBreakPerBlock: land.FineBreakPerBlock,
		AccessPassEnabled: land.AccessPassEnabled,
		AccessTicketItem:  land.AccessTicketItem,
		AccessTicketCost:  land.AccessTicketCost,
	})
	if err != nil {
		return err
	}
	land.MarketTax = out.MarketTax
	land.CurfewEnabled = out.CurfewEnabled
	land.CurfewStart = out.CurfewStart
	land.CurfewEnd = out.CurfewEnd
	land.FineBreakEnabled = out.FineBreakEnabled
	land.FineBreakItem = out.FineBreakItem
	land.FineBreakPerBlock = out.FineBreakPerBlock
	land.AccessPassEnabled = out.AccessPassEnabled
	land.AccessTicketItem = out.AccessTicketItem
	land.AccessTicketCost = out.AccessTicketCost
	return nil
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
	if w.cfg.DayTicks <= 0 || len(w.claims) == 0 {
		return
	}
	day := uint64(w.cfg.DayTicks)

	ids := make([]string, 0, len(w.claims))
	for id := range w.claims {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		c := w.claims[id]
		if c == nil {
			continue
		}
		if c.MaintenanceDueTick == 0 {
			c.MaintenanceDueTick = maintenancepkg.NextDue(nowTick, c.MaintenanceDueTick, day)
			continue
		}
		if nowTick < c.MaintenanceDueTick {
			continue
		}

		status := "PAID"
		if !w.payMaintenance(c) {
			status = "LATE"
			c.MaintenanceStage = maintenancepkg.NextStage(c.MaintenanceStage, false)
		} else {
			c.MaintenanceStage = maintenancepkg.NextStage(c.MaintenanceStage, true)
		}
		c.MaintenanceDueTick = maintenancepkg.NextDue(nowTick, c.MaintenanceDueTick, day)

		// Notify owner agent if present.
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
	}
}

func (w *World) payMaintenance(c *LandClaim) bool {
	if c == nil {
		return false
	}
	owner := strings.TrimSpace(c.Owner)
	if owner == "" {
		return false
	}

	cost := maintenancepkg.EffectiveCost(w.cfg.MaintenanceCost)

	// Prefer org treasury if claim is owned by an org id.
	if org := w.orgByID(owner); org != nil {
		tr := w.orgTreasury(org)
		if tr == nil || !inventorypkg.HasItems(tr, cost) {
			return false
		}
		inventorypkg.DeductItems(tr, cost)
		return true
	}

	a := w.agents[owner]
	if a == nil {
		return false
	}
	if !inventorypkg.HasItems(a.Inventory, cost) {
		return false
	}
	inventorypkg.DeductItems(a.Inventory, cost)
	return true
}
