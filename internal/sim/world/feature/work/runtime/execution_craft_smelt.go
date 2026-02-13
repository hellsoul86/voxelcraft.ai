package runtime

import (
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/tasks"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type WorkExecCraftEnv interface {
	GetRecipe(recipeID string) (catalogs.RecipeDef, bool)
	GetSmeltRecipeByInput(itemID string) (catalogs.RecipeDef, bool)
	NearBlock(pos modelpkg.Vec3i, blockID string, dist int) bool
	OnRecipe(a *modelpkg.Agent, recipeID string, tier int, nowTick uint64)
}

func TickCraft(env WorkExecCraftEnv, a *modelpkg.Agent, wt *tasks.WorkTask, nowTick uint64) {
	rec, ok := env.GetRecipe(wt.RecipeID)
	if !ok {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "unknown recipe"})
		return
	}

	switch rec.Station {
	case "HAND":
	case "CRAFTING_BENCH":
		if !env.NearBlock(a.Pos, "CRAFTING_BENCH", 2) {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "need crafting bench nearby"})
			return
		}
	default:
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "unsupported station"})
		return
	}

	wt.WorkTicks++
	if wt.WorkTicks < rec.TimeTicks {
		return
	}
	wt.WorkTicks = 0

	for _, in := range rec.Inputs {
		if a.Inventory[in.Item] < in.Count {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "missing inputs"})
			return
		}
	}
	for _, in := range rec.Inputs {
		a.Inventory[in.Item] -= in.Count
	}
	for _, out := range rec.Outputs {
		a.Inventory[out.Item] += out.Count
	}
	env.OnRecipe(a, wt.RecipeID, rec.Tier, nowTick)

	wt.Count--
	if wt.Count <= 0 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
	}
}

func TickSmelt(env WorkExecCraftEnv, a *modelpkg.Agent, wt *tasks.WorkTask, nowTick uint64) {
	if !env.NearBlock(a.Pos, "FURNACE", 2) {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "need furnace nearby"})
		return
	}

	rec, ok := env.GetSmeltRecipeByInput(wt.ItemID)
	if !ok {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "unsupported smelt item"})
		return
	}

	wt.WorkTicks++
	if wt.WorkTicks < rec.TimeTicks {
		return
	}
	wt.WorkTicks = 0

	for _, in := range rec.Inputs {
		if a.Inventory[in.Item] < in.Count {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "missing inputs"})
			return
		}
	}
	for _, in := range rec.Inputs {
		a.Inventory[in.Item] -= in.Count
	}
	for _, out := range rec.Outputs {
		a.Inventory[out.Item] += out.Count
	}
	env.OnRecipe(a, rec.RecipeID, rec.Tier, nowTick)

	wt.Count--
	if wt.Count <= 0 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
	}
}
