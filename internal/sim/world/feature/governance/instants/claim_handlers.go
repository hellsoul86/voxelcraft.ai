package instants

import (
	"voxelcraft.ai/internal/protocol"
	claimspkg "voxelcraft.ai/internal/sim/world/feature/governance/claims"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type ClaimInstantEnv interface {
	GetLand(landID string) *modelpkg.LandClaim
	IsLandAdmin(agentID string, land *modelpkg.LandClaim) bool
	BlockNameAt(pos modelpkg.Vec3i) string
	ClaimRecords() []ClaimRecord
	OwnerExists(ownerID string) bool
	AuditClaimEvent(nowTick uint64, actorID string, action string, pos modelpkg.Vec3i, reason string, details map[string]any)
}

func HandleSetPermissions(env ClaimInstantEnv, ar OrgActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "claim env unavailable"))
		return
	}
	if ok, code, msg := claimspkg.ValidateSetPermissionsInput(inst.LandID, inst.Policy); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	land := env.GetLand(inst.LandID)
	if ok, code, msg := ValidateLandAdmin(land != nil, env.IsLandAdmin(a.ID, land)); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
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
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}

func HandleUpgradeClaim(env ClaimInstantEnv, ar OrgActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "claim env unavailable"))
		return
	}
	if ok, code, msg := claimspkg.ValidateUpgradeInput(inst.LandID, inst.Radius); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	land := env.GetLand(inst.LandID)
	if ok, code, msg := ValidateLandAdmin(land != nil, env.IsLandAdmin(a.ID, land)); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	if land.MaintenanceStage >= 1 {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_PERMISSION", "land maintenance stage disallows expansion"))
		return
	}
	target := inst.Radius
	if ok, code, msg := claimspkg.ValidateUpgradeRadius(land.Radius, target); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	if env.BlockNameAt(land.Anchor) != "CLAIM_TOTEM" {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "claim totem missing"))
		return
	}
	cost := claimspkg.UpgradeCost(land.Radius, target)
	if len(cost) == 0 {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BAD_REQUEST", "no upgrade needed"))
		return
	}
	for item, n := range cost {
		if a.Inventory[item] < n {
			a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_RESOURCE", "missing upgrade materials"))
			return
		}
	}
	zones := BuildZones(env.ClaimRecords())
	if claimspkg.UpgradeOverlaps(land.Anchor.X, land.Anchor.Z, target, land.LandID, zones) {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_CONFLICT", "claim overlaps existing land"))
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
	env.AuditClaimEvent(nowTick, a.ID, "CLAIM_UPGRADE", land.Anchor, "UPGRADE_CLAIM", map[string]any{
		"land_id": inst.LandID,
		"from":    from,
		"to":      target,
		"cost":    inventorypkg.EncodeItemPairs(cost),
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "land_id": inst.LandID, "radius": target})
}

func HandleAddMember(env ClaimInstantEnv, ar OrgActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "claim env unavailable"))
		return
	}
	if ok, code, msg := claimspkg.ValidateMemberMutationInput(inst.LandID, inst.MemberID); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	land := env.GetLand(inst.LandID)
	if ok, code, msg := ValidateLandAdmin(land != nil, env.IsLandAdmin(a.ID, land)); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	if land.Members == nil {
		land.Members = map[string]bool{}
	}
	land.Members[inst.MemberID] = true
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}

func HandleRemoveMember(env ClaimInstantEnv, ar OrgActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "claim env unavailable"))
		return
	}
	if ok, code, msg := claimspkg.ValidateMemberMutationInput(inst.LandID, inst.MemberID); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	land := env.GetLand(inst.LandID)
	if ok, code, msg := ValidateLandAdmin(land != nil, env.IsLandAdmin(a.ID, land)); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	if land.Members != nil {
		delete(land.Members, inst.MemberID)
	}
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}

func HandleDeedLand(env ClaimInstantEnv, ar OrgActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "claim env unavailable"))
		return
	}
	if ok, code, msg := claimspkg.ValidateDeedInput(inst.LandID, inst.NewOwner); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	land := env.GetLand(inst.LandID)
	if ok, code, msg := ValidateLandAdmin(land != nil, env.IsLandAdmin(a.ID, land)); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	newOwner := claimspkg.NormalizeNewOwner(inst.NewOwner)
	if newOwner == "" {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad new_owner"))
		return
	}
	if !env.OwnerExists(newOwner) {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "new owner not found"))
		return
	}
	land.Owner = newOwner
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}
