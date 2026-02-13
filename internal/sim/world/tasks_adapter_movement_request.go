package world

import modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"

type movementTaskReqWorldEnv struct {
	w *World
}

func (e movementTaskReqWorldEnv) NewTaskID() string {
	if e.w == nil {
		return ""
	}
	return e.w.newTaskID()
}

func (e movementTaskReqWorldEnv) InBounds(pos Vec3i) bool {
	if e.w == nil {
		return false
	}
	return e.w.chunks.inBounds(pos)
}

func (e movementTaskReqWorldEnv) FollowTargetPos(targetID string) (Vec3i, bool) {
	if e.w == nil {
		return Vec3i{}, false
	}
	return e.w.followTargetPos(targetID)
}

func (e movementTaskReqWorldEnv) SortedAgents() []*modelpkg.Agent {
	if e.w == nil {
		return nil
	}
	return e.w.sortedAgents()
}

func (e movementTaskReqWorldEnv) SurfaceY(x int, z int) int {
	if e.w == nil {
		return 0
	}
	return e.w.surfaceY(x, z)
}

func (e movementTaskReqWorldEnv) BlockSolidAt(pos modelpkg.Vec3i) bool {
	if e.w == nil {
		return false
	}
	return e.w.blockSolid(e.w.chunks.GetBlock(pos))
}

func (e movementTaskReqWorldEnv) LandAt(pos modelpkg.Vec3i) *modelpkg.LandClaim {
	if e.w == nil {
		return nil
	}
	return e.w.landAt(pos)
}

func (e movementTaskReqWorldEnv) LandCoreContains(c *modelpkg.LandClaim, pos modelpkg.Vec3i) bool {
	if e.w == nil {
		return false
	}
	return e.w.landCoreContains(c, pos)
}

func (e movementTaskReqWorldEnv) IsLandMember(agentID string, land *modelpkg.LandClaim) bool {
	if e.w == nil {
		return false
	}
	return e.w.isLandMember(agentID, land)
}

func (e movementTaskReqWorldEnv) OrgByID(id string) *modelpkg.Organization {
	if e.w == nil {
		return nil
	}
	return e.w.orgByID(id)
}

func (e movementTaskReqWorldEnv) TransferAccessTicket(ownerID string, item string, count int) {
	if e.w == nil || ownerID == "" || item == "" || count <= 0 {
		return
	}
	if owner := e.w.agents[ownerID]; owner != nil {
		owner.Inventory[item] += count
		return
	}
	if org := e.w.orgByID(ownerID); org != nil {
		e.w.orgTreasury(org)[item] += count
	}
}

func (e movementTaskReqWorldEnv) RecordDenied(nowTick uint64) {
	if e.w == nil || e.w.stats == nil {
		return
	}
	e.w.stats.RecordDenied(nowTick)
}

func (e movementTaskReqWorldEnv) RecordStructureUsage(agentID string, pos modelpkg.Vec3i, nowTick uint64) {
	if e.w == nil {
		return
	}
	e.w.recordStructureUsage(agentID, pos, nowTick)
}

func (e movementTaskReqWorldEnv) OnBiome(a *modelpkg.Agent, nowTick uint64) {
	if e.w == nil {
		return
	}
	e.w.funOnBiome(a, nowTick)
}
