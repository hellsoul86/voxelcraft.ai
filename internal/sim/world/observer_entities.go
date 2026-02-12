package world

import (
	"voxelcraft.ai/internal/observerproto"
	"voxelcraft.ai/internal/sim/tasks"
	taskspkg "voxelcraft.ai/internal/sim/world/feature/observer/tasks"
	progresspkg "voxelcraft.ai/internal/sim/world/feature/work/progress"
)

func (w *World) observerMoveTaskState(a *Agent, nowTick uint64) *observerproto.TaskState {
	if w == nil || a == nil || a.MoveTask == nil {
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
			t, ok := w.followTargetPos(id)
			return taskspkg.Vec3{X: t.X, Y: t.Y, Z: t.Z}, ok
		},
	)
}

func (w *World) observerWorkTaskState(a *Agent) *observerproto.TaskState {
	if a == nil || a.WorkTask == nil {
		return nil
	}
	wt := a.WorkTask
	return taskspkg.BuildWorkTaskState(string(wt.Kind), w.workProgressForAgent(a, wt))
}

func (w *World) workProgressForAgent(a *Agent, wt *tasks.WorkTask) float64 {
	if a == nil || wt == nil {
		return 0
	}
	switch wt.Kind {
	case tasks.KindMine:
		pos := v3FromTask(wt.BlockPos)
		blockName := w.blockName(w.chunks.GetBlock(pos))
		return progresspkg.MineProgress(wt.WorkTicks, blockName, a.Inventory)
	case tasks.KindCraft:
		rec, ok := w.catalogs.Recipes.ByID[wt.RecipeID]
		if !ok {
			return 0
		}
		return progresspkg.TimedProgress(wt.WorkTicks, rec.TimeTicks)
	case tasks.KindSmelt:
		rec, ok := w.smeltByInput[wt.ItemID]
		if !ok {
			return 0
		}
		return progresspkg.TimedProgress(wt.WorkTicks, rec.TimeTicks)
	case tasks.KindBuildBlueprint:
		bp, ok := w.catalogs.Blueprints.ByID[wt.BlueprintID]
		if !ok {
			return 0
		}
		return progresspkg.BlueprintProgress(wt.BuildIndex, len(bp.Blocks))
	default:
		return 0
	}
}
