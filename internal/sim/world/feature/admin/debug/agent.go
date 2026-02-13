package debug

import (
	"strings"

	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func SetAgentPos(a *modelpkg.Agent, pos modelpkg.Vec3i) bool {
	if a == nil {
		return false
	}
	pos.Y = 0
	a.Pos = pos
	return true
}

func ClearAgentEvents(a *modelpkg.Agent) bool {
	if a == nil {
		return false
	}
	a.Events = nil
	return true
}

func SetAgentVitals(a *modelpkg.Agent, hp, hunger, staminaMilli int) bool {
	if a == nil {
		return false
	}
	if hp >= 0 {
		a.HP = hp
	}
	if hunger >= 0 {
		a.Hunger = hunger
	}
	if staminaMilli >= 0 {
		a.StaminaMilli = staminaMilli
	}
	return true
}

func SetAgentReputation(a *modelpkg.Agent, repTrade, repBuild, repSocial, repLaw int) bool {
	if a == nil {
		return false
	}
	if repTrade >= 0 {
		a.RepTrade = repTrade
	}
	if repBuild >= 0 {
		a.RepBuild = repBuild
	}
	if repSocial >= 0 {
		a.RepSocial = repSocial
	}
	if repLaw >= 0 {
		a.RepLaw = repLaw
	}
	return true
}

func AddInventory(a *modelpkg.Agent, item string, delta int, itemAllowed func(string) bool) bool {
	if a == nil {
		return false
	}
	it := strings.TrimSpace(item)
	if it == "" {
		return false
	}
	if itemAllowed != nil && !itemAllowed(it) {
		return false
	}
	if a.Inventory == nil {
		a.Inventory = map[string]int{}
	}
	if delta == 0 {
		return true
	}
	next := a.Inventory[it] + delta
	if next <= 0 {
		delete(a.Inventory, it)
		return true
	}
	a.Inventory[it] = next
	return true
}
