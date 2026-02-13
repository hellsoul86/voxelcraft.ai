package workrequest

type Env struct {
	NewTaskIDFn        func() string
	ItemEntityExistsFn func(entityID string) bool
	RecipeExistsFn     func(recipeID string) bool
	SmeltExistsFn      func(itemID string) bool
	BlueprintExistsFn  func(blueprintID string) bool
}

func (e Env) NewTaskID() string {
	if e.NewTaskIDFn == nil {
		return ""
	}
	return e.NewTaskIDFn()
}

func (e Env) ItemEntityExists(entityID string) bool {
	if e.ItemEntityExistsFn == nil {
		return false
	}
	return e.ItemEntityExistsFn(entityID)
}

func (e Env) RecipeExists(recipeID string) bool {
	if e.RecipeExistsFn == nil {
		return false
	}
	return e.RecipeExistsFn(recipeID)
}

func (e Env) SmeltExists(itemID string) bool {
	if e.SmeltExistsFn == nil {
		return false
	}
	return e.SmeltExistsFn(itemID)
}

func (e Env) BlueprintExists(blueprintID string) bool {
	if e.BlueprintExistsFn == nil {
		return false
	}
	return e.BlueprintExistsFn(blueprintID)
}
