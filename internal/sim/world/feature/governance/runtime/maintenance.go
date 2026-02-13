package runtime

import (
	"strings"

	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	maintenancepkg "voxelcraft.ai/internal/sim/world/feature/governance/maintenance"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type TickClaimsMaintenanceInput struct {
	NowTick  uint64
	DayTicks int
	Claims   map[string]*modelpkg.LandClaim
}

type TickClaimsMaintenanceHooks struct {
	Pay    func(c *modelpkg.LandClaim) bool
	Notify func(c *modelpkg.LandClaim, status string)
}

func TickClaimsMaintenance(in TickClaimsMaintenanceInput, hooks TickClaimsMaintenanceHooks) {
	if in.DayTicks <= 0 || len(in.Claims) == 0 {
		return
	}
	day := uint64(in.DayTicks)
	for _, id := range SortedClaimIDs(in.Claims) {
		c := in.Claims[id]
		if c == nil {
			continue
		}
		if c.MaintenanceDueTick == 0 {
			c.MaintenanceDueTick = maintenancepkg.NextDue(in.NowTick, c.MaintenanceDueTick, day)
			continue
		}
		if in.NowTick < c.MaintenanceDueTick {
			continue
		}

		status := "PAID"
		paid := hooks.Pay != nil && hooks.Pay(c)
		if !paid {
			status = "LATE"
			c.MaintenanceStage = maintenancepkg.NextStage(c.MaintenanceStage, false)
		} else {
			c.MaintenanceStage = maintenancepkg.NextStage(c.MaintenanceStage, true)
		}
		c.MaintenanceDueTick = maintenancepkg.NextDue(in.NowTick, c.MaintenanceDueTick, day)
		if hooks.Notify != nil {
			hooks.Notify(c, status)
		}
	}
}

type PayMaintenanceInput struct {
	Claim           *modelpkg.LandClaim
	DefaultCost     map[string]int
	GetOrg          func(owner string) *modelpkg.Organization
	OrgTreasury     func(org *modelpkg.Organization) map[string]int
	GetAgent        func(owner string) *modelpkg.Agent
	HasItems        func(inv map[string]int, need map[string]int) bool
	DeductItems     func(inv map[string]int, need map[string]int)
	EffectiveCostFn func(defaultCost map[string]int) map[string]int
}

func PayMaintenance(in PayMaintenanceInput) bool {
	if in.Claim == nil {
		return false
	}
	owner := strings.TrimSpace(in.Claim.Owner)
	if owner == "" {
		return false
	}

	effectiveCost := maintenancepkg.EffectiveCost
	if in.EffectiveCostFn != nil {
		effectiveCost = in.EffectiveCostFn
	}
	cost := effectiveCost(in.DefaultCost)

	hasItems := inventorypkg.HasItems
	if in.HasItems != nil {
		hasItems = in.HasItems
	}
	deductItems := inventorypkg.DeductItems
	if in.DeductItems != nil {
		deductItems = in.DeductItems
	}

	if in.GetOrg != nil && in.OrgTreasury != nil {
		if org := in.GetOrg(owner); org != nil {
			tr := in.OrgTreasury(org)
			if tr == nil || !hasItems(tr, cost) {
				return false
			}
			deductItems(tr, cost)
			return true
		}
	}

	if in.GetAgent == nil {
		return false
	}
	a := in.GetAgent(owner)
	if a == nil {
		return false
	}
	if !hasItems(a.Inventory, cost) {
		return false
	}
	deductItems(a.Inventory, cost)
	return true
}
