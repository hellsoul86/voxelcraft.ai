package smelt

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
)

func TestBuildSmeltByInput(t *testing.T) {
	recipes := map[string]catalogs.RecipeDef{
		"iron": {
			RecipeID:  "iron",
			Station:   "FURNACE",
			TimeTicks: 10,
			Inputs:    []catalogs.ItemCount{{Item: "IRON_ORE", Count: 1}, {Item: "COAL", Count: 1}},
			Outputs:   []catalogs.ItemCount{{Item: "IRON_INGOT", Count: 1}},
		},
		"bench": {
			RecipeID:  "bench",
			Station:   "CRAFTING_BENCH",
			TimeTicks: 5,
			Inputs:    []catalogs.ItemCount{{Item: "PLANK", Count: 4}},
			Outputs:   []catalogs.ItemCount{{Item: "CRAFTING_BENCH", Count: 1}},
		},
	}

	m, err := BuildSmeltByInput(recipes)
	if err != nil {
		t.Fatalf("BuildSmeltByInput err: %v", err)
	}
	if len(m) != 1 {
		t.Fatalf("expected 1 furnace recipe, got %d", len(m))
	}
	if got := m["IRON_ORE"].RecipeID; got != "iron" {
		t.Fatalf("unexpected primary mapping: %q", got)
	}
}

func TestBuildSmeltByInputDuplicatePrimary(t *testing.T) {
	recipes := map[string]catalogs.RecipeDef{
		"a": {
			RecipeID:  "a",
			Station:   "FURNACE",
			TimeTicks: 10,
			Inputs:    []catalogs.ItemCount{{Item: "IRON_ORE", Count: 1}},
			Outputs:   []catalogs.ItemCount{{Item: "IRON_INGOT", Count: 1}},
		},
		"b": {
			RecipeID:  "b",
			Station:   "FURNACE",
			TimeTicks: 10,
			Inputs:    []catalogs.ItemCount{{Item: "IRON_ORE", Count: 1}},
			Outputs:   []catalogs.ItemCount{{Item: "IRON_PLATE", Count: 1}},
		},
	}

	if _, err := BuildSmeltByInput(recipes); err == nil {
		t.Fatalf("expected duplicate primary input error")
	}
}
