package runtime

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	miningpkg "voxelcraft.ai/internal/sim/world/feature/work/mining"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type WorkExecMineEnv interface {
	CanBreakAt(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool
	PermissionsFor(agentID string, pos modelpkg.Vec3i) (*modelpkg.LandClaim, map[string]bool)
	IsLandMember(agentID string, land *modelpkg.LandClaim) bool
	TransferToLandOwner(ownerID string, item string, count int)
	BumpRepLaw(agentID string, delta int)
	RecordDenied(nowTick uint64)

	BlockAt(pos modelpkg.Vec3i) uint16
	AirBlockID() uint16
	BlockName(blockID uint16) string
	SetBlock(pos modelpkg.Vec3i, blockID uint16)
	AuditSetBlock(nowTick uint64, actor string, pos modelpkg.Vec3i, from uint16, to uint16, reason string)
	BlockIDToItem(blockID uint16) string
	SpawnItemEntity(nowTick uint64, actor string, pos modelpkg.Vec3i, item string, count int, reason string) string

	GetContainerAt(pos modelpkg.Vec3i) *modelpkg.Container
	RemoveContainer(pos modelpkg.Vec3i)
	RemoveBoard(pos modelpkg.Vec3i)
	RemoveSign(nowTick uint64, actor string, pos modelpkg.Vec3i, reason string)
	RemoveConveyor(nowTick uint64, actor string, pos modelpkg.Vec3i, reason string)
	RemoveSwitch(nowTick uint64, actor string, pos modelpkg.Vec3i, reason string)
	RemoveClaimByAnchor(nowTick uint64, actor string, pos modelpkg.Vec3i, reason string)

	OnMinedBlockDuringEvent(a *modelpkg.Agent, pos modelpkg.Vec3i, blockName string, nowTick uint64)
}

func TickMine(env WorkExecMineEnv, a *modelpkg.Agent, wt *tasks.WorkTask, nowTick uint64) {
	pos := modelpkg.Vec3i{X: wt.BlockPos.X, Y: wt.BlockPos.Y, Z: wt.BlockPos.Z}
	if modelpkg.Manhattan(a.Pos, pos) > 2 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "too far"})
		return
	}
	if !env.CanBreakAt(a.ID, pos, nowTick) {
		if land, perms := env.PermissionsFor(a.ID, pos); land != nil && !env.IsLandMember(a.ID, land) && !perms["can_break"] &&
			land.FineBreakEnabled && land.FineBreakPerBlock > 0 && strings.TrimSpace(land.FineBreakItem) != "" {
			item := strings.TrimSpace(land.FineBreakItem)
			fine := land.FineBreakPerBlock
			pay := fine
			if have := a.Inventory[item]; have < pay {
				pay = have
			}
			if pay > 0 {
				a.Inventory[item] -= pay
				env.TransferToLandOwner(land.Owner, item, pay)
				a.AddEvent(protocol.Event{"t": nowTick, "type": "FINE", "land_id": land.LandID, "item": item, "count": pay, "reason": "BREAK_DENIED"})
			}
		}
		a.WorkTask = nil
		env.BumpRepLaw(a.ID, -1)
		env.RecordDenied(nowTick)
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "break denied"})
		return
	}

	b := env.BlockAt(pos)
	if b == env.AirBlockID() {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "no block"})
		return
	}
	blockName := env.BlockName(b)

	family := miningpkg.MineToolFamilyForBlock(blockName)
	tier := miningpkg.BestToolTier(a.Inventory, family)
	mineWorkNeeded, mineCost := miningpkg.MineParamsForTier(tier)
	if a.StaminaMilli < mineCost {
		return
	}
	a.StaminaMilli -= mineCost

	wt.WorkTicks++
	if wt.WorkTicks < mineWorkNeeded {
		return
	}

	if blockName != "" {
		switch blockName {
		case "CHEST", "FURNACE", "CONTRACT_TERMINAL":
			c := env.GetContainerAt(pos)
			if c != nil && len(c.Reserved) > 0 {
				a.WorkTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "container has reserved items"})
				return
			}
			if c != nil {
				for item, n := range c.Inventory {
					if n > 0 {
						a.Inventory[item] += n
					}
				}
				if c.Owed != nil {
					if owed := c.Owed[a.ID]; owed != nil {
						for item, n := range owed {
							if n > 0 {
								a.Inventory[item] += n
							}
						}
						delete(c.Owed, a.ID)
					}
				}
				env.RemoveContainer(pos)
			}
		case "BULLETIN_BOARD":
			env.RemoveBoard(pos)
		case "SIGN":
			env.RemoveSign(nowTick, a.ID, pos, "MINE")
		case "CONVEYOR":
			env.RemoveConveyor(nowTick, a.ID, pos, "MINE")
		case "SWITCH":
			env.RemoveSwitch(nowTick, a.ID, pos, "MINE")
		case "CLAIM_TOTEM":
			env.RemoveClaimByAnchor(nowTick, a.ID, pos, "MINE")
		}
	}

	air := env.AirBlockID()
	env.SetBlock(pos, air)
	env.AuditSetBlock(nowTick, a.ID, pos, b, air, "MINE")

	if item := env.BlockIDToItem(b); item != "" {
		_ = env.SpawnItemEntity(nowTick, a.ID, pos, item, 1, "MINE_DROP")
	}
	env.OnMinedBlockDuringEvent(a, pos, blockName, nowTick)
	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}
