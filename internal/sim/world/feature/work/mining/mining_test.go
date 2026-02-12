package mining

import "testing"

func TestMineToolFamilyForBlock(t *testing.T) {
	if got := MineToolFamilyForBlock("DIRT"); got != ToolFamilyShovel {
		t.Fatalf("expected shovel family for DIRT, got %v", got)
	}
	if got := MineToolFamilyForBlock("LOG"); got != ToolFamilyAxe {
		t.Fatalf("expected axe family for LOG, got %v", got)
	}
	if got := MineToolFamilyForBlock("STONE"); got != ToolFamilyPickaxe {
		t.Fatalf("expected pickaxe family for STONE, got %v", got)
	}
}

func TestBestToolTier(t *testing.T) {
	inv := map[string]int{
		"WOOD_PICKAXE":  1,
		"STONE_PICKAXE": 1,
		"IRON_AXE":      1,
	}
	if got := BestToolTier(inv, ToolFamilyPickaxe); got != 2 {
		t.Fatalf("expected stone tier (2), got %d", got)
	}
	if got := BestToolTier(inv, ToolFamilyAxe); got != 3 {
		t.Fatalf("expected iron tier (3), got %d", got)
	}
	if got := BestToolTier(inv, ToolFamilyShovel); got != 0 {
		t.Fatalf("expected no shovel tier (0), got %d", got)
	}
}

func TestMineParamsForTier(t *testing.T) {
	work, cost := MineParamsForTier(0)
	if work != 10 || cost != 15 {
		t.Fatalf("tier0 mismatch: got work=%d cost=%d", work, cost)
	}
	work, cost = MineParamsForTier(2)
	if work != 6 || cost != 11 {
		t.Fatalf("tier2 mismatch: got work=%d cost=%d", work, cost)
	}
	work, cost = MineParamsForTier(3)
	if work != 4 || cost != 9 {
		t.Fatalf("tier3 mismatch: got work=%d cost=%d", work, cost)
	}
}
