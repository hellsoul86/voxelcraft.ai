package world

import "voxelcraft.ai/internal/sim/tasks"

func (w *World) workProgressForAgent(a *Agent, wt *tasks.WorkTask) float64 {
	if a == nil || wt == nil {
		return 0
	}
	switch wt.Kind {
	case tasks.KindMine:
		pos := v3FromTask(wt.BlockPos)
		blockName := w.blockName(w.chunks.GetBlock(pos))
		family := mineToolFamilyForBlock(blockName)
		tier := bestToolTier(a.Inventory, family)
		workNeeded, _ := mineParamsForTier(tier)
		if workNeeded <= 0 {
			return 0
		}
		return clamp01(float64(wt.WorkTicks) / float64(workNeeded))
	case tasks.KindCraft:
		rec, ok := w.catalogs.Recipes.ByID[wt.RecipeID]
		if !ok || rec.TimeTicks <= 0 {
			return 0
		}
		return clamp01(float64(wt.WorkTicks) / float64(rec.TimeTicks))
	case tasks.KindSmelt:
		rec, ok := w.smeltByInput[wt.ItemID]
		if !ok || rec.TimeTicks <= 0 {
			return 0
		}
		return clamp01(float64(wt.WorkTicks) / float64(rec.TimeTicks))
	case tasks.KindBuildBlueprint:
		bp, ok := w.catalogs.Blueprints.ByID[wt.BlueprintID]
		if !ok || len(bp.Blocks) == 0 {
			return 0
		}
		return clamp01(float64(wt.BuildIndex) / float64(len(bp.Blocks)))
	default:
		return 0
	}
}
