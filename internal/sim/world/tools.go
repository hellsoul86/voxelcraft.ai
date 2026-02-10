package world

type toolFamily int

const (
	toolFamilyNone toolFamily = iota
	toolFamilyPickaxe
	toolFamilyAxe
	toolFamilyShovel
)

func mineToolFamilyForBlock(blockName string) toolFamily {
	switch blockName {
	case "DIRT", "GRASS", "SAND", "GRAVEL":
		return toolFamilyShovel
	case "LOG", "PLANK":
		return toolFamilyAxe
	default:
		// Default: treat everything else as "pickaxe preferred".
		return toolFamilyPickaxe
	}
}

func bestToolTier(inv map[string]int, family toolFamily) int {
	if len(inv) == 0 {
		return 0
	}
	switch family {
	case toolFamilyPickaxe:
		if inv["IRON_PICKAXE"] > 0 {
			return 3
		}
		if inv["STONE_PICKAXE"] > 0 {
			return 2
		}
		if inv["WOOD_PICKAXE"] > 0 {
			return 1
		}
	case toolFamilyAxe:
		if inv["IRON_AXE"] > 0 {
			return 3
		}
		if inv["STONE_AXE"] > 0 {
			return 2
		}
		if inv["WOOD_AXE"] > 0 {
			return 1
		}
	case toolFamilyShovel:
		if inv["IRON_SHOVEL"] > 0 {
			return 3
		}
		if inv["STONE_SHOVEL"] > 0 {
			return 2
		}
		if inv["WOOD_SHOVEL"] > 0 {
			return 1
		}
	}
	return 0
}

func mineParamsForTier(tier int) (workNeeded int, staminaCost int) {
	// Defaults.
	workNeeded = 10
	staminaCost = 15

	switch tier {
	case 3: // iron
		return 4, 9
	case 2: // stone
		return 6, 11
	case 1: // wood
		return 8, 13
	default:
		return workNeeded, staminaCost
	}
}
