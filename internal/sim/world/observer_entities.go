package world

import (
	"voxelcraft.ai/internal/observerproto"
	"voxelcraft.ai/internal/sim/tasks"
)

func (w *World) observerMoveTaskState(a *Agent, nowTick uint64) *observerproto.TaskState {
	if w == nil || a == nil || a.MoveTask == nil {
		return nil
	}
	mt := a.MoveTask

	target := v3FromTask(mt.Target)
	if mt.Kind == tasks.KindFollow {
		if t, ok := w.followTargetPos(mt.TargetID); ok {
			target = t
		}
		want := int(ceil(mt.Distance))
		if want < 1 {
			want = 1
		}
		d := distXZ(a.Pos, target)
		prog := 0.0
		if d <= want {
			prog = 1.0
		}
		eta := d - want
		if eta < 0 {
			eta = 0
		}
		return &observerproto.TaskState{
			Kind:     string(mt.Kind),
			TargetID: mt.TargetID,
			Target:   target.ToArray(),
			Progress: prog,
			EtaTicks: eta,
		}
	}

	start := v3FromTask(mt.StartPos)

	// Match the agent OBS semantics: completion is within tolerance, and progress/eta are based on effective XZ distance.
	want := int(ceil(mt.Tolerance))
	if want < 1 {
		want = 1
	}
	distStart := distXZ(start, target)
	distCur := distXZ(a.Pos, target)
	totalEff := distStart - want
	if totalEff < 0 {
		totalEff = 0
	}
	remEff := distCur - want
	if remEff < 0 {
		remEff = 0
	}
	prog := 1.0
	if totalEff > 0 {
		prog = float64(totalEff-remEff) / float64(totalEff)
		if prog < 0 {
			prog = 0
		} else if prog > 1 {
			prog = 1
		}
	}
	eta := remEff
	return &observerproto.TaskState{
		Kind:     string(mt.Kind),
		Target:   target.ToArray(),
		Progress: prog,
		EtaTicks: eta,
	}
}

func (w *World) observerWorkTaskState(a *Agent) *observerproto.TaskState {
	if a == nil || a.WorkTask == nil {
		return nil
	}
	wt := a.WorkTask
	return &observerproto.TaskState{
		Kind:     string(wt.Kind),
		Progress: w.workProgressForAgent(a, wt),
	}
}
