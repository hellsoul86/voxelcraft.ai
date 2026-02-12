package lifecycle

import "testing"

func TestBuildDeadline(t *testing.T) {
	if got := BuildDeadline(100, 0, 0, 6000); got != 6100 {
		t.Fatalf("expected 6100, got %d", got)
	}
	if got := BuildDeadline(100, 999, 10, 6000); got != 999 {
		t.Fatalf("expected explicit deadline 999, got %d", got)
	}
}

func TestValidatePostInput(t *testing.T) {
	if ok, _, _ := ValidatePostInput("GATHER", map[string]int{"STONE": 1}, map[string]int{"COAL": 1}); !ok {
		t.Fatalf("expected valid")
	}
	if ok, code, _ := ValidatePostInput("GATHER", nil, map[string]int{"COAL": 1}); ok || code == "" {
		t.Fatalf("expected invalid requirements")
	}
}

func TestScaleDeposit(t *testing.T) {
	got := ScaleDeposit(map[string]int{"IRON": 2}, 3)
	if got["IRON"] != 6 {
		t.Fatalf("expected scaled deposit 6, got %#v", got)
	}
}

func TestCanSubmit(t *testing.T) {
	if !CanSubmit("DELIVER", true, false) {
		t.Fatalf("expected deliver submit")
	}
	if CanSubmit("BUILD", true, false) {
		t.Fatalf("expected build submit blocked")
	}
	if !NeedsRequirementsConsumption("GATHER") || NeedsRequirementsConsumption("BUILD") {
		t.Fatalf("unexpected needs-requirements mapping")
	}
}
