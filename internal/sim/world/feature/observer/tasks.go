package observer

import (
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/world/logic/observerprogress"
)

type TaskVec3 struct {
	X int
	Y int
	Z int
}

func (v TaskVec3) ToArray() [3]int { return [3]int{v.X, v.Y, v.Z} }

type MoveTaskInput struct {
	TaskID    string
	Kind      string
	Target    TaskVec3
	StartPos  TaskVec3
	TargetID  string
	Distance  float64
	Tolerance float64
}

type WorkTaskInput struct {
	TaskID   string
	Kind     string
	Progress float64
}

type BuildTasksInput struct {
	SelfPos TaskVec3
	Move    *MoveTaskInput
	Work    *WorkTaskInput
}

func BuildTasks(in BuildTasksInput, resolveFollowTarget func(string) (TaskVec3, bool)) []protocol.TaskObs {
	out := make([]protocol.TaskObs, 0, 2)
	if in.Move != nil {
		mt := in.Move
		target := mt.Target
		if mt.Kind == "FOLLOW" && resolveFollowTarget != nil {
			if t, ok := resolveFollowTarget(mt.TargetID); ok {
				target = t
			}
			prog, eta := observerprogress.FollowProgress(
				observerprogress.Vec3{X: in.SelfPos.X, Y: in.SelfPos.Y, Z: in.SelfPos.Z},
				observerprogress.Vec3{X: target.X, Y: target.Y, Z: target.Z},
				mt.Distance,
			)
			out = append(out, protocol.TaskObs{
				TaskID:   mt.TaskID,
				Kind:     mt.Kind,
				Progress: prog,
				Target:   target.ToArray(),
				EtaTicks: eta,
			})
		} else {
			prog, eta := observerprogress.MoveProgress(
				observerprogress.Vec3{X: mt.StartPos.X, Y: mt.StartPos.Y, Z: mt.StartPos.Z},
				observerprogress.Vec3{X: in.SelfPos.X, Y: in.SelfPos.Y, Z: in.SelfPos.Z},
				observerprogress.Vec3{X: target.X, Y: target.Y, Z: target.Z},
				mt.Tolerance,
			)
			out = append(out, protocol.TaskObs{
				TaskID:   mt.TaskID,
				Kind:     mt.Kind,
				Progress: prog,
				Target:   target.ToArray(),
				EtaTicks: eta,
			})
		}
	}
	if in.Work != nil {
		out = append(out, protocol.TaskObs{
			TaskID:   in.Work.TaskID,
			Kind:     in.Work.Kind,
			Progress: in.Work.Progress,
		})
	}
	return out
}
