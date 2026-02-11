package world

import (
	"sort"
	"strings"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/world/feature/economy"
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
			c.MaintenanceDueTick = nowTick + day
			continue
		}
		if nowTick < c.MaintenanceDueTick {
			continue
		}

		status := "PAID"
		if !w.payMaintenance(c) {
			status = "LATE"
			if c.MaintenanceStage < 2 {
				c.MaintenanceStage++
			}
		} else {
			c.MaintenanceStage = 0
		}
		c.MaintenanceDueTick += day

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

	cost := w.cfg.MaintenanceCost
	if len(cost) == 0 {
		// Defensive default (should be set by cfg.applyDefaults).
		cost = map[string]int{"IRON_INGOT": 1, "COAL": 1}
	}

	// Prefer org treasury if claim is owned by an org id.
	if org := w.orgByID(owner); org != nil {
		tr := w.orgTreasury(org)
		if tr == nil || !economy.HasItems(tr, cost) {
			return false
		}
		deductItems(tr, cost)
		return true
	}

	a := w.agents[owner]
	if a == nil {
		return false
	}
	if !economy.HasItems(a.Inventory, cost) {
		return false
	}
	deductItems(a.Inventory, cost)
	return true
}

func deductItems(inv map[string]int, cost map[string]int) {
	for item, c := range cost {
		inv[item] -= c
		if inv[item] <= 0 {
			delete(inv, item)
		}
	}
}
