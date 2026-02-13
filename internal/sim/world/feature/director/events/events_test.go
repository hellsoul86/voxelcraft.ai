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

func TestBuildInstantiatePlan(t *testing.T) {
	p := BuildInstantiatePlan("CRYSTAL_RIFT", nil)
	if !p.NeedsCenter || p.Radius != 32 || p.Spawn != SpawnCrystalRift {
		t.Fatalf("unexpected crystal plan: %+v", p)
	}

	p = BuildInstantiatePlan("BUILDER_EXPO", map[string]any{"theme": "TOWER"})
	if p.Spawn != SpawnNoticeBoard || p.Headline == "" || p.Body == "" {
		t.Fatalf("unexpected expo plan: %+v", p)
	}
	if p.Body != "主题: TOWER。完成蓝图建造并展示。" {
		t.Fatalf("expo body mismatch: %q", p.Body)
	}

	p = BuildInstantiatePlan("UNKNOWN", nil)
	if p.NeedsCenter || p.Spawn != SpawnNone {
		t.Fatalf("unknown event should produce empty plan: %+v", p)
	}
}
