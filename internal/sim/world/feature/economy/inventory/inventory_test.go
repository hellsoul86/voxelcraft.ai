package inventory

import "testing"

func TestDeductItems(t *testing.T) {
	inv := map[string]int{
		"IRON_INGOT": 3,
		"COAL":       1,
	}
	DeductItems(inv, map[string]int{
		"IRON_INGOT": 2,
		"COAL":       1,
		"":           5,
	})
	if inv["IRON_INGOT"] != 1 {
		t.Fatalf("expected IRON_INGOT=1, got %d", inv["IRON_INGOT"])
	}
	if _, ok := inv["COAL"]; ok {
		t.Fatalf("expected COAL removed, got %#v", inv)
	}
}
