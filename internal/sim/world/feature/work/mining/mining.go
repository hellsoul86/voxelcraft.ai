package mining

type ToolFamily int

const (
	ToolFamilyNone ToolFamily = iota
	ToolFamilyPickaxe
	ToolFamilyAxe
	ToolFamilyShovel
)

func MineToolFamilyForBlock(blockName string) ToolFamily {
	switch blockName {
	case "DIRT", "GRASS", "SAND", "GRAVEL":
		return ToolFamilyShovel
	case "LOG", "PLANK":
		return ToolFamilyAxe
	default:
		return ToolFamilyPickaxe
	}
}

func BestToolTier(inv map[string]int, family ToolFamily) int {
	if len(inv) == 0 {
		return 0
	}
	switch family {
	case ToolFamilyPickaxe:
		if inv["IRON_PICKAXE"] > 0 {
			return 3
		}
		if inv["STONE_PICKAXE"] > 0 {
			return 2
		}
		if inv["WOOD_PICKAXE"] > 0 {
			return 1
		}
	case ToolFamilyAxe:
		if inv["IRON_AXE"] > 0 {
			return 3
		}
		if inv["STONE_AXE"] > 0 {
			return 2
		}
		if inv["WOOD_AXE"] > 0 {
			return 1
		}
	case ToolFamilyShovel:
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

func MineParamsForTier(tier int) (workNeeded int, staminaCost int) {
	workNeeded = 10
	staminaCost = 15
	switch tier {
	case 3:
		return 4, 9
	case 2:
		return 6, 11
	case 1:
		return 8, 13
	default:
		return workNeeded, staminaCost
	}
}
