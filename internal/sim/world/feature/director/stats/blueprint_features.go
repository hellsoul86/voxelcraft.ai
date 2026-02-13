package stats

import "voxelcraft.ai/internal/sim/catalogs"

type BlueprintFeatures struct {
	UniqueBlockTypes int
	HasStorage       bool
	HasLight         bool
	HasWorkshop      bool
	HasGovernance    bool
}

func ExtractBlueprintFeatures(blocks []catalogs.BPBlock) BlueprintFeatures {
	unique := map[string]bool{}
	hasStorage := false
	hasLight := false
	hasWorkshop := false
	hasGov := false

	for _, b := range blocks {
		unique[b.Block] = true
		switch b.Block {
		case "CHEST":
			hasStorage = true
		case "TORCH":
			hasLight = true
		case "CRAFTING_BENCH", "FURNACE":
			hasWorkshop = true
		case "BULLETIN_BOARD", "CONTRACT_TERMINAL", "CLAIM_TOTEM", "SIGN":
			hasGov = true
		}
	}

	return BlueprintFeatures{
		UniqueBlockTypes: len(unique),
		HasStorage:       hasStorage,
		HasLight:         hasLight,
		HasWorkshop:      hasWorkshop,
		HasGovernance:    hasGov,
	}
}
