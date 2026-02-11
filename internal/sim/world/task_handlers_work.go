package world

import (
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
)

func handleTaskMine(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if !w.cfg.AllowMine {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_PERMISSION", "mining disabled in this world"))
		return
	}
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.BlockPos[1] != 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "2D world requires y==0"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindMine,
		BlockPos:    tasks.Vec3i{X: tr.BlockPos[0], Y: 0, Z: tr.BlockPos[2]},
		StartedTick: nowTick,
		WorkTicks:   0,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskGather(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.TargetID == "" {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	if w.items[tr.TargetID] == nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "item entity not found"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindGather,
		TargetID:    tr.TargetID,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskPlace(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if !w.cfg.AllowPlace {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_PERMISSION", "placing disabled in this world"))
		return
	}
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.ItemID == "" {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing item_id"))
		return
	}
	if tr.BlockPos[1] != 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "2D world requires y==0"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindPlace,
		ItemID:      tr.ItemID,
		BlockPos:    tasks.Vec3i{X: tr.BlockPos[0], Y: 0, Z: tr.BlockPos[2]},
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskOpen(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.TargetID == "" {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindOpen,
		TargetID:    tr.TargetID,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskTransfer(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.Src == "" || tr.Dst == "" || tr.ItemID == "" || tr.Count <= 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing src/dst/item_id/count"))
		return
	}
	taskID := w.newTaskID()
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

func handleTaskCraft(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.RecipeID == "" || tr.Count <= 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing recipe_id/count"))
		return
	}
	if _, ok := w.catalogs.Recipes.ByID[tr.RecipeID]; !ok {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "unknown recipe"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindCraft,
		RecipeID:    tr.RecipeID,
		Count:       tr.Count,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskSmelt(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.ItemID == "" || tr.Count <= 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing item_id/count"))
		return
	}
	if _, ok := w.smeltByInput[tr.ItemID]; !ok {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "unsupported smelt item"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindSmelt,
		ItemID:      tr.ItemID,
		Count:       tr.Count,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func (w *World) systemWorkImpl(nowTick uint64) {
	for _, a := range w.sortedAgents() {
		wt := a.WorkTask
		if wt == nil {
			continue
		}

		switch wt.Kind {
		case tasks.KindMine:
			w.tickMine(a, wt, nowTick)
		case tasks.KindGather:
			w.tickGather(a, wt, nowTick)
		case tasks.KindPlace:
			w.tickPlace(a, wt, nowTick)
		case tasks.KindOpen:
			w.tickOpen(a, wt, nowTick)
		case tasks.KindTransfer:
			w.tickTransfer(a, wt, nowTick)
		case tasks.KindCraft:
			w.tickCraft(a, wt, nowTick)
		case tasks.KindSmelt:
			w.tickSmelt(a, wt, nowTick)
		case tasks.KindBuildBlueprint:
			w.tickBuildBlueprint(a, wt, nowTick)
		}
	}
}
