package maintenance

import "testing"

func TestEffectiveMaintenanceCost(t *testing.T) {
	got := EffectiveCost(nil)
	if got["IRON_INGOT"] != 1 || got["COAL"] != 1 {
		t.Fatalf("expected default maintenance cost, got %#v", got)
	}
}

func TestNextMaintenanceDue(t *testing.T) {
	if got := NextDue(100, 0, 6000); got != 6100 {
		t.Fatalf("expected 6100, got %d", got)
	}
	if got := NextDue(100, 7000, 6000); got != 13000 {
		t.Fatalf("expected 13000, got %d", got)
	}
}

func TestNextMaintenanceStage(t *testing.T) {
	if got := NextStage(2, true); got != 0 {
		t.Fatalf("expected reset to 0, got %d", got)
	}
	if got := NextStage(0, false); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	if got := NextStage(2, false); got != 2 {
		t.Fatalf("expected cap at 2, got %d", got)
	}
}
