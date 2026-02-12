package events

import "testing"

func TestMineOutcome(t *testing.T) {
	got := MineOutcome("CRYSTAL_RIFT", "CRYSTAL_ORE")
	if !got.OK || got.GrantItem != "CRYSTAL_SHARD" || got.GrantCount != 1 {
		t.Fatalf("unexpected crystal outcome: %+v", got)
	}
	if got := MineOutcome("DEEP_VEIN", "STONE"); got.OK {
		t.Fatalf("stone should not trigger deep vein reward")
	}
}

func TestOpenContainerOutcome(t *testing.T) {
	got := OpenContainerOutcome("BANDIT_CAMP", "CHEST")
	if !got.OK || got.Risk <= 0 || got.GoalKind == "" {
		t.Fatalf("unexpected bandit outcome: %+v", got)
	}
	if got := OpenContainerOutcome("RUINS_GATE", "FURNACE"); got.OK {
		t.Fatalf("non-chest should not trigger outcome")
	}
}
