package world

import (
	"fmt"
	"sort"
	"strings"

	"voxelcraft.ai/internal/sim/catalogs"
)

func buildSmeltByInput(recipes map[string]catalogs.RecipeDef) (map[string]catalogs.RecipeDef, error) {
	ids := make([]string, 0, len(recipes))
	for id := range recipes {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := map[string]catalogs.RecipeDef{}
	for _, id := range ids {
		r := recipes[id]
		if strings.TrimSpace(r.Station) != "FURNACE" {
			continue
		}
		if len(r.Inputs) == 0 {
			return nil, fmt.Errorf("furnace recipe %q: missing inputs", r.RecipeID)
		}
		if len(r.Outputs) == 0 {
			return nil, fmt.Errorf("furnace recipe %q: missing outputs", r.RecipeID)
		}
		if r.TimeTicks <= 0 {
			return nil, fmt.Errorf("furnace recipe %q: invalid time_ticks=%d", r.RecipeID, r.TimeTicks)
		}
		key := strings.TrimSpace(r.Inputs[0].Item)
		if key == "" {
			return nil, fmt.Errorf("furnace recipe %q: empty primary input item", r.RecipeID)
		}
		if prev, ok := out[key]; ok {
			return nil, fmt.Errorf("duplicate furnace recipe primary input %q: %q and %q", key, prev.RecipeID, r.RecipeID)
		}
		out[key] = r
	}
	return out, nil
}
