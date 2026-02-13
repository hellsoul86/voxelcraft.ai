package movement

import modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"

type Env struct {
	NewTaskIDFn            func() string
	InBoundsFn             func(pos modelpkg.Vec3i) bool
	FollowTargetPosFn      func(targetID string) (modelpkg.Vec3i, bool)
	SortedAgentsFn         func() []*modelpkg.Agent
	SurfaceYFn             func(x int, z int) int
	BlockSolidAtFn         func(pos modelpkg.Vec3i) bool
	LandAtFn               func(pos modelpkg.Vec3i) *modelpkg.LandClaim
	LandCoreContainsFn     func(c *modelpkg.LandClaim, pos modelpkg.Vec3i) bool
	IsLandMemberFn         func(agentID string, land *modelpkg.LandClaim) bool
	OrgByIDFn              func(id string) *modelpkg.Organization
	TransferAccessTicketFn func(ownerID string, item string, count int)
	RecordDeniedFn         func(nowTick uint64)
	RecordStructureUsageFn func(agentID string, pos modelpkg.Vec3i, nowTick uint64)
	OnBiomeFn              func(a *modelpkg.Agent, nowTick uint64)
}

func (e Env) NewTaskID() string {
	if e.NewTaskIDFn == nil {
		return ""
	}
	return e.NewTaskIDFn()
}

func (e Env) InBounds(pos modelpkg.Vec3i) bool {
	if e.InBoundsFn == nil {
		return false
	}
	return e.InBoundsFn(pos)
}

func (e Env) FollowTargetPos(targetID string) (modelpkg.Vec3i, bool) {
	if e.FollowTargetPosFn == nil {
		return modelpkg.Vec3i{}, false
	}
	return e.FollowTargetPosFn(targetID)
}

func (e Env) SortedAgents() []*modelpkg.Agent {
	if e.SortedAgentsFn == nil {
		return nil
	}
	return e.SortedAgentsFn()
}

func (e Env) SurfaceY(x int, z int) int {
	if e.SurfaceYFn == nil {
		return 0
	}
	return e.SurfaceYFn(x, z)
}

func (e Env) BlockSolidAt(pos modelpkg.Vec3i) bool {
	if e.BlockSolidAtFn == nil {
		return false
	}
	return e.BlockSolidAtFn(pos)
}

func (e Env) LandAt(pos modelpkg.Vec3i) *modelpkg.LandClaim {
	if e.LandAtFn == nil {
		return nil
	}
	return e.LandAtFn(pos)
}

func (e Env) LandCoreContains(c *modelpkg.LandClaim, pos modelpkg.Vec3i) bool {
	if e.LandCoreContainsFn == nil {
		return false
	}
	return e.LandCoreContainsFn(c, pos)
}

func (e Env) IsLandMember(agentID string, land *modelpkg.LandClaim) bool {
	if e.IsLandMemberFn == nil {
		return false
	}
	return e.IsLandMemberFn(agentID, land)
}

func (e Env) OrgByID(id string) *modelpkg.Organization {
	if e.OrgByIDFn == nil {
		return nil
	}
	return e.OrgByIDFn(id)
}

func (e Env) TransferAccessTicket(ownerID string, item string, count int) {
	if e.TransferAccessTicketFn != nil {
		e.TransferAccessTicketFn(ownerID, item, count)
	}
}

func (e Env) RecordDenied(nowTick uint64) {
	if e.RecordDeniedFn != nil {
		e.RecordDeniedFn(nowTick)
	}
}

func (e Env) RecordStructureUsage(agentID string, pos modelpkg.Vec3i, nowTick uint64) {
	if e.RecordStructureUsageFn != nil {
		e.RecordStructureUsageFn(agentID, pos, nowTick)
	}
}

func (e Env) OnBiome(a *modelpkg.Agent, nowTick uint64) {
	if e.OnBiomeFn != nil {
		e.OnBiomeFn(a, nowTick)
	}
}
