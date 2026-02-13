package runtime

import (
	"voxelcraft.ai/internal/protocol"
	taskspkg "voxelcraft.ai/internal/sim/world/feature/observer/tasks"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type BuildTasksFromWorldInput struct {
	Agent    *modelpkg.Agent
	Progress float64
}

func BuildTasksFromWorld(in BuildTasksFromWorldInput, followTargetPos func(id string) (modelpkg.Vec3i, bool)) []protocol.TaskObs {
	a := in.Agent
	if a == nil {
		return nil
	}

	var moveIn *taskspkg.MoveInput
	if a.MoveTask != nil {
		moveIn = &taskspkg.MoveInput{
			TaskID:    a.MoveTask.TaskID,
			Kind:      string(a.MoveTask.Kind),
			Target:    taskspkg.Vec3{X: a.MoveTask.Target.X, Y: a.MoveTask.Target.Y, Z: a.MoveTask.Target.Z},
			StartPos:  taskspkg.Vec3{X: a.MoveTask.StartPos.X, Y: a.MoveTask.StartPos.Y, Z: a.MoveTask.StartPos.Z},
			TargetID:  a.MoveTask.TargetID,
			Distance:  a.MoveTask.Distance,
			Tolerance: a.MoveTask.Tolerance,
		}
	}
	var workIn *taskspkg.WorkInput
	if a.WorkTask != nil {
		workIn = &taskspkg.WorkInput{
			TaskID:   a.WorkTask.TaskID,
			Kind:     string(a.WorkTask.Kind),
			Progress: in.Progress,
		}
	}
	return taskspkg.BuildTasks(taskspkg.BuildInput{
		SelfPos: taskspkg.Vec3{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
		Move:    moveIn,
		Work:    workIn,
	}, func(id string) (taskspkg.Vec3, bool) {
		if followTargetPos == nil {
			return taskspkg.Vec3{}, false
		}
		if t, ok := followTargetPos(id); ok {
			return taskspkg.Vec3{X: t.X, Y: t.Y, Z: t.Z}, true
		}
		return taskspkg.Vec3{}, false
	})
}
