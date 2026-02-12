package tasks

import (
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/world/logic/observerprogress"
)

type Vec3 struct {
	X int
	Y int
	Z int
}

func (v Vec3) ToArray() [3]int { return [3]int{v.X, v.Y, v.Z} }

type MoveInput struct {
	TaskID    string
	Kind      string
	Target    Vec3
	StartPos  Vec3
	TargetID  string
	Distance  float64
	Tolerance float64
}

type WorkInput struct {
	TaskID   string
	Kind     string
	Progress float64
}

type BuildInput struct {
	SelfPos Vec3
	Move    *MoveInput
	Work    *WorkInput
}

func BuildTasks(in BuildInput, resolveFollowTarget func(string) (Vec3, bool)) []protocol.TaskObs {
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
