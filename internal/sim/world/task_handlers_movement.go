package world

import (
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	paramspkg "voxelcraft.ai/internal/sim/world/feature/movement/params"
)

func handleTaskStop(_ *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	a.MoveTask = nil
	a.AddEvent(actionResult(nowTick, tr.ID, true, "", "stopped"))
}

func handleTaskMoveTo(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.MoveTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "movement task slot occupied"))
		return
	}
	// Reject targets outside the world boundary to avoid agents wandering into the "void"
	// (GetBlock returns AIR outside BoundaryR, and we don't generate chunks there).
	tx, ty, tz := paramspkg.NormalizeTarget(tr.Target[0], tr.Target[2])
	if !w.chunks.inBounds(Vec3i{X: tx, Y: ty, Z: tz}) {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "out of bounds"))
		return
	}
	taskID := w.newTaskID()
	a.MoveTask = &tasks.MovementTask{
		TaskID:      taskID,
		Kind:        tasks.KindMoveTo,
		Target:      tasks.Vec3i{X: tx, Y: ty, Z: tz},
		Tolerance:   tr.Tolerance,
		StartPos:    tasks.Vec3i{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{
		"t":       nowTick,
		"type":    "ACTION_RESULT",
		"ref":     tr.ID,
		"ok":      true,
		"task_id": taskID,
	})
}

func handleTaskFollow(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.MoveTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "movement task slot occupied"))
		return
	}
	if tr.TargetID == "" {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	dist := paramspkg.ClampFollowDistance(tr.Distance)
	target, ok := w.followTargetPos(tr.TargetID)
	if !ok {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "target not found"))
		return
	}
	taskID := w.newTaskID()
	a.MoveTask = &tasks.MovementTask{
		TaskID:      taskID,
		Kind:        tasks.KindFollow,
		Target:      v3ToTask(target),
		TargetID:    tr.TargetID,
		Distance:    dist,
		StartPos:    tasks.Vec3i{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{
		"t":       nowTick,
		"type":    "ACTION_RESULT",
		"ref":     tr.ID,
		"ok":      true,
		"task_id": taskID,
	})
}
