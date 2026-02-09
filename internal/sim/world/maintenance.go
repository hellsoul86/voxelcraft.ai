package world

import (
	"sort"
	"strings"

	"voxelcraft.ai/internal/protocol"
)

const (
	maintenanceIron = 1
	maintenanceCoal = 1
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

	// Prefer org treasury if claim is owned by an org id.
	if org := w.orgByID(owner); org != nil {
		if org.Treasury["IRON_INGOT"] < maintenanceIron || org.Treasury["COAL"] < maintenanceCoal {
			return false
		}
		org.Treasury["IRON_INGOT"] -= maintenanceIron
		org.Treasury["COAL"] -= maintenanceCoal
		return true
	}

	a := w.agents[owner]
	if a == nil {
		return false
	}
	if a.Inventory["IRON_INGOT"] < maintenanceIron || a.Inventory["COAL"] < maintenanceCoal {
		return false
	}
	a.Inventory["IRON_INGOT"] -= maintenanceIron
	a.Inventory["COAL"] -= maintenanceCoal
	return true
}
