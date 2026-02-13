package runtime

import (
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type ActionResultFn func(tick uint64, ref string, ok bool, code string, message string) protocol.Event

type WorkRequestEnv interface {
	NewTaskID() string
	ItemEntityExists(entityID string) bool
	RecipeExists(recipeID string) bool
	SmeltExists(itemID string) bool
}

func HandleTaskMine(env WorkRequestEnv, ar ActionResultFn, a *modelpkg.Agent, tr protocol.TaskReq, nowTick uint64, allowMine bool) {
	if !allowMine {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_NO_PERMISSION", "mining disabled in this world"))
		return
	}
	if a.WorkTask != nil {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.BlockPos[1] != 0 {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_INVALID_TARGET", "2D world requires y==0"))
		return
	}
	taskID := env.NewTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindMine,
		BlockPos:    tasks.Vec3i{X: tr.BlockPos[0], Y: 0, Z: tr.BlockPos[2]},
		StartedTick: nowTick,
		WorkTicks:   0,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func HandleTaskGather(env WorkRequestEnv, ar ActionResultFn, a *modelpkg.Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.TargetID == "" {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	if env == nil || !env.ItemEntityExists(tr.TargetID) {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_INVALID_TARGET", "item entity not found"))
		return
	}
	taskID := env.NewTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindGather,
		TargetID:    tr.TargetID,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func HandleTaskPlace(env WorkRequestEnv, ar ActionResultFn, a *modelpkg.Agent, tr protocol.TaskReq, nowTick uint64, allowPlace bool) {
	if !allowPlace {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_NO_PERMISSION", "placing disabled in this world"))
		return
	}
	if a.WorkTask != nil {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.ItemID == "" {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing item_id"))
		return
	}
	if tr.BlockPos[1] != 0 {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_INVALID_TARGET", "2D world requires y==0"))
		return
	}
	taskID := env.NewTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindPlace,
		ItemID:      tr.ItemID,
		BlockPos:    tasks.Vec3i{X: tr.BlockPos[0], Y: 0, Z: tr.BlockPos[2]},
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func HandleTaskOpen(env WorkRequestEnv, ar ActionResultFn, a *modelpkg.Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.TargetID == "" {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	taskID := env.NewTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindOpen,
		TargetID:    tr.TargetID,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func HandleTaskTransfer(env WorkRequestEnv, ar ActionResultFn, a *modelpkg.Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.Src == "" || tr.Dst == "" || tr.ItemID == "" || tr.Count <= 0 {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing src/dst/item_id/count"))
		return
	}
	taskID := env.NewTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:       taskID,
		Kind:         tasks.KindTransfer,
		SrcContainer: tr.Src,
		DstContainer: tr.Dst,
		ItemID:       tr.ItemID,
		Count:        tr.Count,
		StartedTick:  nowTick,
		WorkTicks:    0,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func HandleTaskCraft(env WorkRequestEnv, ar ActionResultFn, a *modelpkg.Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.RecipeID == "" || tr.Count <= 0 {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing recipe_id/count"))
		return
	}
	if env == nil || !env.RecipeExists(tr.RecipeID) {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_INVALID_TARGET", "unknown recipe"))
		return
	}
	taskID := env.NewTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindCraft,
		RecipeID:    tr.RecipeID,
		Count:       tr.Count,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func HandleTaskSmelt(env WorkRequestEnv, ar ActionResultFn, a *modelpkg.Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.ItemID == "" || tr.Count <= 0 {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing item_id/count"))
		return
	}
	if env == nil || !env.SmeltExists(tr.ItemID) {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_INVALID_TARGET", "unsupported smelt item"))
		return
	}
	taskID := env.NewTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindSmelt,
		ItemID:      tr.ItemID,
		Count:       tr.Count,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}
