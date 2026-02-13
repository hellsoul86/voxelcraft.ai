package world

import workrequestctxpkg "voxelcraft.ai/internal/sim/world/featurectx/workrequest"

func newWorkTaskReqEnv(w *World) workrequestctxpkg.Env {
	return workrequestctxpkg.Env{
		NewTaskIDFn: w.newTaskID,
		ItemEntityExistsFn: func(entityID string) bool {
			return w.items[entityID] != nil
		},
		RecipeExistsFn: func(recipeID string) bool {
			_, ok := w.catalogs.Recipes.ByID[recipeID]
			return ok
		},
		SmeltExistsFn: func(itemID string) bool {
			_, ok := w.smeltByInput[itemID]
			return ok
		},
		BlueprintExistsFn: func(blueprintID string) bool {
			_, ok := w.catalogs.Blueprints.ByID[blueprintID]
			return ok
		},
	}
}
