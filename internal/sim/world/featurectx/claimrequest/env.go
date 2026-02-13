package claimrequest

import modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"

type Env struct {
	InBoundsFn          func(pos modelpkg.Vec3i) bool
	CanBuildAtFn        func(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool
	ClaimsFn            func() []*modelpkg.LandClaim
	BlockAtFn           func(pos modelpkg.Vec3i) uint16
	AirBlockIDFn        func() uint16
	ClaimTotemBlockIDFn func() (uint16, bool)
	SetBlockFn          func(pos modelpkg.Vec3i, blockID uint16)
	AuditSetBlockFn     func(nowTick uint64, actor string, pos modelpkg.Vec3i, from uint16, to uint16, reason string)
	NewLandIDFn         func(owner string) string
	WorldTypeFn         func() string
	DayTicksFn          func() int
	PutClaimFn          func(c *modelpkg.LandClaim)
}

func (e Env) InBounds(pos modelpkg.Vec3i) bool {
	if e.InBoundsFn == nil {
		return false
	}
	return e.InBoundsFn(pos)
}

func (e Env) CanBuildAt(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool {
	if e.CanBuildAtFn == nil {
		return false
	}
	return e.CanBuildAtFn(agentID, pos, nowTick)
}

func (e Env) Claims() []*modelpkg.LandClaim {
	if e.ClaimsFn == nil {
		return nil
	}
	return e.ClaimsFn()
}

func (e Env) BlockAt(pos modelpkg.Vec3i) uint16 {
	if e.BlockAtFn == nil {
		return 0
	}
	return e.BlockAtFn(pos)
}

func (e Env) AirBlockID() uint16 {
	if e.AirBlockIDFn == nil {
		return 0
	}
	return e.AirBlockIDFn()
}

func (e Env) ClaimTotemBlockID() (uint16, bool) {
	if e.ClaimTotemBlockIDFn == nil {
		return 0, false
	}
	return e.ClaimTotemBlockIDFn()
}

func (e Env) SetBlock(pos modelpkg.Vec3i, blockID uint16) {
	if e.SetBlockFn != nil {
		e.SetBlockFn(pos, blockID)
	}
}

func (e Env) AuditSetBlock(nowTick uint64, actor string, pos modelpkg.Vec3i, from uint16, to uint16, reason string) {
	if e.AuditSetBlockFn != nil {
		e.AuditSetBlockFn(nowTick, actor, pos, from, to, reason)
	}
}

func (e Env) NewLandID(owner string) string {
	if e.NewLandIDFn == nil {
		return ""
	}
	return e.NewLandIDFn(owner)
}

func (e Env) WorldType() string {
	if e.WorldTypeFn == nil {
		return ""
	}
	return e.WorldTypeFn()
}

func (e Env) DayTicks() int {
	if e.DayTicksFn == nil {
		return 0
	}
	return e.DayTicksFn()
}

func (e Env) PutClaim(c *modelpkg.LandClaim) {
	if e.PutClaimFn != nil {
		e.PutClaimFn(c)
	}
}
