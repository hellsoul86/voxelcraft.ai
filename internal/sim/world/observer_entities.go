package world

import (
	"voxelcraft.ai/internal/observerproto"
	"voxelcraft.ai/internal/sim/tasks"
	featurework "voxelcraft.ai/internal/sim/world/feature/work"
	"voxelcraft.ai/internal/sim/world/logic/observerprogress"
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
		prog, eta := observerprogress.FollowProgress(
			observerprogress.Vec3{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
			observerprogress.Vec3{X: target.X, Y: target.Y, Z: target.Z},
			mt.Distance,
		)
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
	prog, eta := observerprogress.MoveProgress(
		observerprogress.Vec3{X: start.X, Y: start.Y, Z: start.Z},
		observerprogress.Vec3{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
		observerprogress.Vec3{X: target.X, Y: target.Y, Z: target.Z},
		mt.Tolerance,
	)
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

func (w *World) workProgressForAgent(a *Agent, wt *tasks.WorkTask) float64 {
	if a == nil || wt == nil {
		return 0
	}
	switch wt.Kind {
	case tasks.KindMine:
		pos := v3FromTask(wt.BlockPos)
		blockName := w.blockName(w.chunks.GetBlock(pos))
		return featurework.MineProgress(wt.WorkTicks, blockName, a.Inventory)
	case tasks.KindCraft:
		rec, ok := w.catalogs.Recipes.ByID[wt.RecipeID]
		if !ok {
			return 0
		}
		return featurework.TimedProgress(wt.WorkTicks, rec.TimeTicks)
	case tasks.KindSmelt:
		rec, ok := w.smeltByInput[wt.ItemID]
		if !ok {
			return 0
		}
		return featurework.TimedProgress(wt.WorkTicks, rec.TimeTicks)
	case tasks.KindBuildBlueprint:
		bp, ok := w.catalogs.Blueprints.ByID[wt.BlueprintID]
		if !ok {
			return 0
		}
		return featurework.BlueprintProgress(wt.BuildIndex, len(bp.Blocks))
	default:
		return 0
	}
}
