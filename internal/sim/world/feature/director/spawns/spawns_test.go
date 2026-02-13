package spawns

import "testing"

func TestSquareCount(t *testing.T) {
	got := Square(Pos{X: 10, Y: 0, Z: 10}, 2)
	if len(got) != 25 {
		t.Fatalf("Square len=%d, want 25", len(got))
	}
}

func TestDiamondCount(t *testing.T) {
	got := Diamond(Pos{X: 0, Y: 0, Z: 0}, 2)
	// 1 + 4 + 8
	if len(got) != 13 {
		t.Fatalf("Diamond len=%d, want 13", len(got))
	}
}

func TestRingSquareCount(t *testing.T) {
	got := RingSquare(Pos{X: 0, Y: 0, Z: 0}, 2)
	if len(got) != 16 {
		t.Fatalf("RingSquare len=%d, want 16", len(got))
	}
}

func TestCrystalRiftPlan(t *testing.T) {
	center := Pos{X: 10, Y: 0, Z: -5}
	plan := CrystalRiftPlan(center)
	if plan.Center == nil || *plan.Center != center {
		t.Fatalf("center mismatch: %+v", plan.Center)
	}
	if len(plan.Placements) != 25 {
		t.Fatalf("placements=%d want 25", len(plan.Placements))
	}
	for _, p := range plan.Placements {
		if p.Block != "CRYSTAL_ORE" {
			t.Fatalf("unexpected block %q", p.Block)
		}
	}
}

func TestRuinsGatePlanLoot(t *testing.T) {
	center := Pos{X: 0, Y: 0, Z: 0}
	plan := RuinsGatePlan(center)
	if len(plan.Placements) != 9 {
		t.Fatalf("placements=%d want 9", len(plan.Placements))
	}
	if len(plan.Containers) != 1 {
		t.Fatalf("containers=%d want 1", len(plan.Containers))
	}
	c := plan.Containers[0]
	if c.Type != "CHEST" || c.Pos != center {
		t.Fatalf("unexpected container: %+v", c)
	}
	if c.Items["CRYSTAL_SHARD"] != 2 || c.Items["IRON_INGOT"] != 4 || c.Items["COPPER_INGOT"] != 4 {
		t.Fatalf("unexpected loot: %+v", c.Items)
	}
}

func TestBanditCampPlan(t *testing.T) {
	center := Pos{X: 2, Y: 0, Z: 3}
	plan := BanditCampPlan(center)
	if plan.Center == nil || *plan.Center != center {
		t.Fatalf("center mismatch: %+v", plan.Center)
	}
	if len(plan.Containers) != 1 {
		t.Fatalf("containers=%d want 1", len(plan.Containers))
	}
	if len(plan.Signs) != 1 || plan.Signs[0].Text != "BANDIT CAMP" {
		t.Fatalf("unexpected signs: %+v", plan.Signs)
	}
	foundAir := false
	foundChest := false
	foundBrick := false
	for _, p := range plan.Placements {
		switch p.Block {
		case "AIR":
			foundAir = true
		case "CHEST":
			foundChest = true
		case "BRICK":
			foundBrick = true
		}
	}
	if !foundAir || !foundChest || !foundBrick {
		t.Fatalf("expected AIR/CHEST/BRICK placements, got %+v", plan.Placements)
	}
}
