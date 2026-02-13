package claims

import (
	"voxelcraft.ai/internal/protocol"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
	"voxelcraft.ai/internal/sim/world/logic/mathx"
)

type ClaimTaskActionResultFn func(tick uint64, ref string, ok bool, code string, message string) protocol.Event

type ClaimTaskEnv interface {
	InBounds(pos modelpkg.Vec3i) bool
	CanBuildAt(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool
	Claims() []*modelpkg.LandClaim
	BlockAt(pos modelpkg.Vec3i) uint16
	AirBlockID() uint16
	ClaimTotemBlockID() (uint16, bool)
	SetBlock(pos modelpkg.Vec3i, blockID uint16)
	AuditSetBlock(nowTick uint64, actor string, pos modelpkg.Vec3i, from uint16, to uint16, reason string)
	NewLandID(owner string) string
	WorldType() string
	DayTicks() int
	PutClaim(c *modelpkg.LandClaim)
}

func HandleTaskClaimLand(env ClaimTaskEnv, ar ClaimTaskActionResultFn, a *modelpkg.Agent, tr protocol.TaskReq, nowTick uint64, allowClaims bool) {
	if !allowClaims {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_NO_PERMISSION", "claims disabled in this world"))
		return
	}
	if env == nil {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_INTERNAL", "claim env unavailable"))
		return
	}
	r := tr.Radius
	if r <= 0 {
		r = 32
	}
	if r > 128 {
		r = 128
	}
	if tr.Anchor[1] != 0 {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_INVALID_TARGET", "2D world requires y==0"))
		return
	}
	anchor := modelpkg.Vec3i{X: tr.Anchor[0], Y: tr.Anchor[1], Z: tr.Anchor[2]}
	if !env.InBounds(anchor) {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_INVALID_TARGET", "out of bounds"))
		return
	}
	if !env.CanBuildAt(a.ID, anchor, nowTick) {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_NO_PERMISSION", "cannot claim here"))
		return
	}
	if a.Inventory["BATTERY"] < 1 || a.Inventory["CRYSTAL_SHARD"] < 1 {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_NO_RESOURCE", "need BATTERY + CRYSTAL_SHARD"))
		return
	}
	for _, c := range env.Claims() {
		if c == nil {
			continue
		}
		if mathx.AbsInt(anchor.X-c.Anchor.X) <= r+c.Radius && mathx.AbsInt(anchor.Z-c.Anchor.Z) <= r+c.Radius {
			a.AddEvent(ar(nowTick, tr.ID, false, "E_CONFLICT", "claim overlaps existing land"))
			return
		}
	}
	if env.BlockAt(anchor) != env.AirBlockID() {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_BLOCKED", "anchor occupied"))
		return
	}
	totemID, ok := env.ClaimTotemBlockID()
	if !ok {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_INTERNAL", "missing CLAIM_TOTEM block"))
		return
	}

	a.Inventory["BATTERY"]--
	a.Inventory["CRYSTAL_SHARD"]--

	env.SetBlock(anchor, totemID)
	env.AuditSetBlock(nowTick, a.ID, anchor, env.AirBlockID(), totemID, "CLAIM_LAND")

	landID := env.NewLandID(a.ID)
	claimType := DefaultClaimTypeForWorld(env.WorldType())
	baseFlags := DefaultFlags(claimType)
	due := uint64(0)
	if env.DayTicks() > 0 {
		due = nowTick + uint64(env.DayTicks())
	}
	env.PutClaim(&modelpkg.LandClaim{
		LandID:    landID,
		Owner:     a.ID,
		ClaimType: claimType,
		Anchor:    anchor,
		Radius:    r,
		Flags: modelpkg.ClaimFlags{
			AllowBuild:  baseFlags.AllowBuild,
			AllowBreak:  baseFlags.AllowBreak,
			AllowDamage: baseFlags.AllowDamage,
			AllowTrade:  baseFlags.AllowTrade,
		},
		MaintenanceDueTick: due,
		MaintenanceStage:   0,
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "land_id": landID})
}
