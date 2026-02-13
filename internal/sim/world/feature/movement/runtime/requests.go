package runtime

import (
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	paramspkg "voxelcraft.ai/internal/sim/world/feature/movement/params"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type MovementActionResultFn func(tick uint64, ref string, ok bool, code string, message string) protocol.Event

type MovementRequestEnv interface {
	NewTaskID() string
	InBounds(pos modelpkg.Vec3i) bool
	FollowTargetPos(targetID string) (modelpkg.Vec3i, bool)
}

func HandleTaskMoveTo(env MovementRequestEnv, ar MovementActionResultFn, a *modelpkg.Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.MoveTask != nil {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_CONFLICT", "movement task slot occupied"))
		return
	}

	tx, ty, tz := paramspkg.NormalizeTarget(tr.Target[0], tr.Target[2])
	if !env.InBounds(modelpkg.Vec3i{X: tx, Y: ty, Z: tz}) {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_INVALID_TARGET", "out of bounds"))
		return
	}
	taskID := env.NewTaskID()
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

func HandleTaskFollow(env MovementRequestEnv, ar MovementActionResultFn, a *modelpkg.Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.MoveTask != nil {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_CONFLICT", "movement task slot occupied"))
		return
	}
	if tr.TargetID == "" {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	dist := paramspkg.ClampFollowDistance(tr.Distance)
	target, ok := env.FollowTargetPos(tr.TargetID)
	if !ok {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_INVALID_TARGET", "target not found"))
		return
	}
	taskID := env.NewTaskID()
	a.MoveTask = &tasks.MovementTask{
		TaskID:      taskID,
		Kind:        tasks.KindFollow,
		Target:      tasks.Vec3i{X: target.X, Y: target.Y, Z: target.Z},
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
