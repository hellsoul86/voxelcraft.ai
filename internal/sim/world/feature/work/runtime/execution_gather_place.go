package runtime

import (
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type WorkExecGatherPlaceEnv interface {
	GetItemEntity(id string) *modelpkg.ItemEntity
	CanPickupItemEntity(agentID string, pos modelpkg.Vec3i) bool
	RemoveItemEntity(nowTick uint64, actor string, id string, reason string)

	InBounds(pos modelpkg.Vec3i) bool
	CanBuildAt(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool
	RecordDenied(nowTick uint64)
	BumpRepLaw(agentID string, delta int)
	BlockAt(pos modelpkg.Vec3i) uint16
	AirBlockID() uint16
	ItemPlaceAs(itemID string) (string, bool)
	BlockIDByName(blockName string) (uint16, bool)
	SetBlock(pos modelpkg.Vec3i, blockID uint16)
	AuditSetBlock(nowTick uint64, actor string, pos modelpkg.Vec3i, from uint16, to uint16, reason string)
	EnsureContainerForPlacedBlock(pos modelpkg.Vec3i, blockName string)
	EnsureConveyorFromYaw(pos modelpkg.Vec3i, yaw int)
}

func TickGather(env WorkExecGatherPlaceEnv, a *modelpkg.Agent, wt *tasks.WorkTask, nowTick uint64) {
	id := wt.TargetID
	if id == "" {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BAD_REQUEST", "message": "missing target_id"})
		return
	}
	e := env.GetItemEntity(id)
	if e == nil || e.Item == "" || e.Count <= 0 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "item entity not found"})
		return
	}
	if modelpkg.Manhattan(a.Pos, e.Pos) > 2 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "too far"})
		return
	}
	if !env.CanPickupItemEntity(a.ID, e.Pos) {
		a.WorkTask = nil
		env.BumpRepLaw(a.ID, -1)
		env.RecordDenied(nowTick)
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "pickup denied"})
		return
	}

	a.Inventory[e.Item] += e.Count
	env.RemoveItemEntity(nowTick, a.ID, id, "GATHER")

	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}

func TickPlace(env WorkExecGatherPlaceEnv, a *modelpkg.Agent, wt *tasks.WorkTask, nowTick uint64) {
	pos := modelpkg.Vec3i{X: wt.BlockPos.X, Y: wt.BlockPos.Y, Z: wt.BlockPos.Z}
	if !env.InBounds(pos) {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "out of bounds"})
		return
	}
	if !env.CanBuildAt(a.ID, pos, nowTick) {
		a.WorkTask = nil
		env.BumpRepLaw(a.ID, -1)
		env.RecordDenied(nowTick)
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "build denied"})
		return
	}
	if env.BlockAt(pos) != env.AirBlockID() {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "space occupied"})
		return
	}
	if wt.ItemID == "" || a.Inventory[wt.ItemID] < 1 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "missing item"})
		return
	}

	blockName := wt.ItemID
	if placeAs, ok := env.ItemPlaceAs(wt.ItemID); ok && placeAs != "" {
		blockName = placeAs
	}
	bid, ok := env.BlockIDByName(blockName)
	if !ok {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "item not placeable"})
		return
	}

	a.Inventory[wt.ItemID]--
	env.SetBlock(pos, bid)
	env.AuditSetBlock(nowTick, a.ID, pos, env.AirBlockID(), bid, "PLACE")
	env.EnsureContainerForPlacedBlock(pos, blockName)
	if blockName == "CONVEYOR" {
		env.EnsureConveyorFromYaw(pos, a.Yaw)
	}

	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}
