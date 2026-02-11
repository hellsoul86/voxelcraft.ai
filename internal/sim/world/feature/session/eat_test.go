package session

import "testing"

func TestNormalizeConsumeCount(t *testing.T) {
	if got := NormalizeConsumeCount(0); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	if got := NormalizeConsumeCount(-3); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	if got := NormalizeConsumeCount(4); got != 4 {
		t.Fatalf("expected 4, got %d", got)
	}
}

func TestApplyFood(t *testing.T) {
	start := EatState{
		HP:           10,
		Hunger:       4,
		StaminaMilli: 300,
	}
	got := ApplyFood(start, 3, 2)
	if got.HP != 16 {
		t.Fatalf("expected hp=16, got %d", got.HP)
	}
	if got.Hunger != 16 {
		t.Fatalf("expected hunger=16, got %d", got.Hunger)
	}
	if got.StaminaMilli != 600 {
		t.Fatalf("expected stamina=600, got %d", got.StaminaMilli)
	}
}

func TestApplyFoodCaps(t *testing.T) {
	start := EatState{
		HP:           19,
		Hunger:       19,
		StaminaMilli: 980,
	}
	got := ApplyFood(start, 2, 2)
	if got.HP != 20 || got.Hunger != 20 || got.StaminaMilli != 1000 {
		t.Fatalf("expected caps (20,20,1000), got (%d,%d,%d)", got.HP, got.Hunger, got.StaminaMilli)
	}
}
