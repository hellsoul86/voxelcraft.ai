package tasks

import (
	"voxelcraft.ai/internal/observerproto"
	"voxelcraft.ai/internal/sim/world/logic/observerprogress"
)

type MoveStateInput struct {
	Kind      string
	Target    Vec3
	StartPos  Vec3
	TargetID  string
	Distance  float64
	Tolerance float64
}

func BuildMoveTaskState(self Vec3, in MoveStateInput, resolveFollowTarget func(string) (Vec3, bool)) *observerproto.TaskState {
	target := in.Target
	if in.Kind == "FOLLOW" {
		if resolveFollowTarget != nil {
			if t, ok := resolveFollowTarget(in.TargetID); ok {
				target = t
			}
		}
		prog, eta := observerprogress.FollowProgress(
			observerprogress.Vec3{X: self.X, Y: self.Y, Z: self.Z},
			observerprogress.Vec3{X: target.X, Y: target.Y, Z: target.Z},
			in.Distance,
		)
		return &observerproto.TaskState{
			Kind:     in.Kind,
			TargetID: in.TargetID,
			Target:   target.ToArray(),
			Progress: prog,
			EtaTicks: eta,
		}
	}

	prog, eta := observerprogress.MoveProgress(
		observerprogress.Vec3{X: in.StartPos.X, Y: in.StartPos.Y, Z: in.StartPos.Z},
		observerprogress.Vec3{X: self.X, Y: self.Y, Z: self.Z},
		observerprogress.Vec3{X: target.X, Y: target.Y, Z: target.Z},
		in.Tolerance,
	)
	return &observerproto.TaskState{
		Kind:     in.Kind,
		Target:   target.ToArray(),
		Progress: prog,
		EtaTicks: eta,
	}
}

func BuildWorkTaskState(kind string, progress float64) *observerproto.TaskState {
	return &observerproto.TaskState{
		Kind:     kind,
		Progress: progress,
	}
}

