package world

import (
	"sort"
	"strings"

	"voxelcraft.ai/internal/protocol"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	maintenancepkg "voxelcraft.ai/internal/sim/world/feature/governance/maintenance"
)

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
