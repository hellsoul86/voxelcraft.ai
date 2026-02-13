package stats

import "testing"

func TestSocialFunFactor(t *testing.T) {
	if got := SocialFunFactor(0); got != 0.5 {
		t.Fatalf("rep 0 factor=%v want 0.5", got)
	}
	if got := SocialFunFactor(500); got != 1.0 {
		t.Fatalf("rep 500 factor=%v want 1.0", got)
	}
}

func TestStructureID(t *testing.T) {
	got := StructureID("A1", 42, "hut", 1, 0, -2)
	want := "STRUCT_A1_42_hut_1_0_-2"
	if got != want {
		t.Fatalf("id=%q want %q", got, want)
	}
}

func TestCreationScore(t *testing.T) {
	score := CreationScore(CreationScoreInput{
		UniqueBlockTypes: 3,
		HasStorage:       true,
		HasLight:         true,
		HasWorkshop:      true,
		HasGovernance:    false,
		Stable:           true,
		Users:            4,
	})
	if score <= 0 {
		t.Fatalf("expected positive score, got %d", score)
	}
}

func TestStructureUniqueUsers(t *testing.T) {
	used := map[string]uint64{
		"builder": 100,
		"u1":      95,
		"u2":      50,
		"":        100,
	}
	got := StructureUniqueUsers(used, "builder", 100, 10)
	if got != 1 {
		t.Fatalf("unique users=%d want 1", got)
	}
}

func TestIsStructureStable(t *testing.T) {
	positions := []Vec3{
		{X: 0, Y: 2, Z: 0},
		{X: 1, Y: 2, Z: 0},
	}
	if IsStructureStable(positions, func(x, y, z int) bool { return false }) {
		t.Fatalf("expected unsupported structure to be unstable")
	}
	if !IsStructureStable(positions, func(x, y, z int) bool { return x == 0 && y == 1 && z == 0 }) {
		t.Fatalf("expected structure with ground support to be stable")
	}
}
