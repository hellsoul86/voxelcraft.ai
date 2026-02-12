package world

import (
	"voxelcraft.ai/internal/protocol"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	claimspkg "voxelcraft.ai/internal/sim/world/feature/governance/claims"
)

func handleInstantSetPermissions(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := claimspkg.ValidateSetPermissionsInput(inst.LandID, inst.Policy); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
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
	next := claimspkg.ApplyPolicyFlags(claimspkg.Flags{
		AllowBuild:  land.Flags.AllowBuild,
		AllowBreak:  land.Flags.AllowBreak,
		AllowDamage: land.Flags.AllowDamage,
		AllowTrade:  land.Flags.AllowTrade,
	}, inst.Policy)
	land.Flags.AllowBuild = next.AllowBuild
	land.Flags.AllowBreak = next.AllowBreak
	land.Flags.AllowDamage = next.AllowDamage
	land.Flags.AllowTrade = next.AllowTrade
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantUpgradeClaim(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := claimspkg.ValidateUpgradeInput(inst.LandID, inst.Radius); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
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
	if ok, code, msg := claimspkg.ValidateUpgradeRadius(land.Radius, target); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	if w.blockName(w.chunks.GetBlock(land.Anchor)) != "CLAIM_TOTEM" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "claim totem missing"))
		return
	}

	cost := claimspkg.UpgradeCost(land.Radius, target)
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

	zones := make([]claimspkg.Zone, 0, len(w.claims))
	for _, c := range w.claims {
		if c == nil {
			continue
		}
		zones = append(zones, claimspkg.Zone{
			LandID:  c.LandID,
			AnchorX: c.Anchor.X,
			AnchorZ: c.Anchor.Z,
			Radius:  c.Radius,
		})
	}
	if claimspkg.UpgradeOverlaps(land.Anchor.X, land.Anchor.Z, target, land.LandID, zones) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "claim overlaps existing land"))
		return
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
		"cost":    inventorypkg.EncodeItemPairs(cost),
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "land_id": inst.LandID, "radius": target})
}

func handleInstantAddMember(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := claimspkg.ValidateMemberMutationInput(inst.LandID, inst.MemberID); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
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
	if ok, code, msg := claimspkg.ValidateMemberMutationInput(inst.LandID, inst.MemberID); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
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
	if ok, code, msg := claimspkg.ValidateDeedInput(inst.LandID, inst.NewOwner); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
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
	newOwner := claimspkg.NormalizeNewOwner(inst.NewOwner)
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
