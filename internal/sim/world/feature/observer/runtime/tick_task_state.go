package runtime

import (
	"voxelcraft.ai/internal/observerproto"
	taskspkg "voxelcraft.ai/internal/sim/world/feature/observer/tasks"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func BuildMoveTaskStateFromWorld(a *modelpkg.Agent, followTargetPos func(id string) (modelpkg.Vec3i, bool)) *observerproto.TaskState {
	if a == nil || a.MoveTask == nil {
		return nil
	}
	mt := a.MoveTask
	return taskspkg.BuildMoveTaskState(
		taskspkg.Vec3{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
		taskspkg.MoveStateInput{
			Kind:      string(mt.Kind),
			Target:    taskspkg.Vec3{X: mt.Target.X, Y: mt.Target.Y, Z: mt.Target.Z},
			StartPos:  taskspkg.Vec3{X: mt.StartPos.X, Y: mt.StartPos.Y, Z: mt.StartPos.Z},
			TargetID:  mt.TargetID,
			Distance:  mt.Distance,
			Tolerance: mt.Tolerance,
		},
		func(id string) (taskspkg.Vec3, bool) {
			if followTargetPos == nil {
				return taskspkg.Vec3{}, false
			}
			t, ok := followTargetPos(id)
			return taskspkg.Vec3{X: t.X, Y: t.Y, Z: t.Z}, ok
		},
	)
}

func BuildWorkTaskStateFromWorld(a *modelpkg.Agent, progress float64) *observerproto.TaskState {
	if a == nil || a.WorkTask == nil {
		return nil
	}
	return taskspkg.BuildWorkTaskState(string(a.WorkTask.Kind), progress)
}
