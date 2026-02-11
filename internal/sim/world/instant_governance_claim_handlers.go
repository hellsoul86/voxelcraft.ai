package world

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/world/feature/economy"
	"voxelcraft.ai/internal/sim/world/logic/mathx"
)

func handleInstantSetPermissions(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.LandID == "" || inst.Policy == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/policy"))
		return
	}
	land := w.claims[inst.LandID]
	if land == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !w.isLandAdmin(a.ID, land) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not land admin"))
		return
	}
	if v, ok := inst.Policy["allow_build"]; ok {
		land.Flags.AllowBuild = v
	}
	if v, ok := inst.Policy["allow_break"]; ok {
		land.Flags.AllowBreak = v
	}
	if v, ok := inst.Policy["allow_damage"]; ok {
		land.Flags.AllowDamage = v
	}
	if v, ok := inst.Policy["allow_trade"]; ok {
		land.Flags.AllowTrade = v
	}
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantUpgradeClaim(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.LandID == "" || inst.Radius <= 0 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/radius"))
		return
	}
	land := w.claims[inst.LandID]
	if land == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !w.isLandAdmin(a.ID, land) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not land admin"))
		return
	}
	if land.MaintenanceStage >= 1 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "land maintenance stage disallows expansion"))
		return
	}
	target := inst.Radius
	if target != 64 && target != 128 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "radius must be 64 or 128"))
		return
	}
	if target <= land.Radius {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "radius must increase"))
		return
	}
	if w.blockName(w.chunks.GetBlock(land.Anchor)) != "CLAIM_TOTEM" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "claim totem missing"))
		return
	}

	cost := map[string]int{}
	addCost := func(item string, n int) {
		if item == "" || n <= 0 {
			return
		}
		cost[item] += n
	}
	if land.Radius < 64 && target >= 64 {
		addCost("BATTERY", 1)
		addCost("CRYSTAL_SHARD", 2)
	}
	if land.Radius < 128 && target >= 128 {
		addCost("BATTERY", 2)
		addCost("CRYSTAL_SHARD", 4)
	}
	if len(cost) == 0 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "no upgrade needed"))
		return
	}
	for item, n := range cost {
		if a.Inventory[item] < n {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "missing upgrade materials"))
			return
		}
	}

	for _, c := range w.claims {
		if c == nil || c.LandID == land.LandID {
			continue
		}
		if mathx.AbsInt(land.Anchor.X-c.Anchor.X) <= target+c.Radius && mathx.AbsInt(land.Anchor.Z-c.Anchor.Z) <= target+c.Radius {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "claim overlaps existing land"))
			return
		}
	}

	for item, n := range cost {
		a.Inventory[item] -= n
		if a.Inventory[item] <= 0 {
			delete(a.Inventory, item)
		}
	}
	from := land.Radius
	land.Radius = target
	w.auditEvent(nowTick, a.ID, "CLAIM_UPGRADE", land.Anchor, "UPGRADE_CLAIM", map[string]any{
		"land_id": inst.LandID,
		"from":    from,
		"to":      target,
		"cost":    economy.EncodeItemPairs(cost),
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "land_id": inst.LandID, "radius": target})
}

func handleInstantAddMember(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.LandID == "" || inst.MemberID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/member_id"))
		return
	}
	land := w.claims[inst.LandID]
	if land == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !w.isLandAdmin(a.ID, land) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not land admin"))
		return
	}
	if land.Members == nil {
		land.Members = map[string]bool{}
	}
	land.Members[inst.MemberID] = true
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantRemoveMember(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.LandID == "" || inst.MemberID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/member_id"))
		return
	}
	land := w.claims[inst.LandID]
	if land == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !w.isLandAdmin(a.ID, land) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not land admin"))
		return
	}
	if land.Members != nil {
		delete(land.Members, inst.MemberID)
	}
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantDeedLand(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.LandID == "" || inst.NewOwner == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/new_owner"))
		return
	}
	land := w.claims[inst.LandID]
	if land == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !w.isLandAdmin(a.ID, land) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not land admin"))
		return
	}
	newOwner := strings.TrimSpace(inst.NewOwner)
	if newOwner == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad new_owner"))
		return
	}
	if w.agents[newOwner] == nil && w.orgByID(newOwner) == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "new owner not found"))
		return
	}
	land.Owner = newOwner
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}
