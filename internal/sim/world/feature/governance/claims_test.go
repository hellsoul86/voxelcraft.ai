package governance

import "testing"

func TestDefaultClaimFlags_Homestead(t *testing.T) {
	f := DefaultClaimFlags("HOMESTEAD")
	if f.AllowBuild || f.AllowBreak || f.AllowDamage || f.AllowTrade {
		t.Fatalf("expected strict homestead visitor flags, got %+v", f)
	}
}

func TestCanActionWithCurfew(t *testing.T) {
	// Curfew between 0.2 and 0.4 blocks action.
	if CanActionWithCurfew(true, true, 0.3, 0.2, 0.4) {
		t.Fatalf("expected action denied during curfew window")
	}
	if !CanActionWithCurfew(true, true, 0.5, 0.2, 0.4) {
		t.Fatalf("expected action allowed outside curfew window")
	}
}

func TestCoreRadiusAndContains(t *testing.T) {
	if got := CoreRadius(12, 16); got != 12 {
		t.Fatalf("core radius cap mismatch: got %d want 12", got)
	}
	if got := CoreRadius(32, 0); got != 16 {
		t.Fatalf("core radius default mismatch: got %d want 16", got)
	}
	if !CoreContains(10, 10, 12, 11, 3) {
		t.Fatalf("expected point inside core")
	}
	if CoreContains(10, 10, 20, 20, 3) {
		t.Fatalf("expected point outside core")
	}
}

func TestTimeOfDay(t *testing.T) {
	if got := TimeOfDay(300, 600); got != 0.5 {
		t.Fatalf("time of day mismatch: got %f want 0.5", got)
	}
	if got := TimeOfDay(12, 0); got != 0 {
		t.Fatalf("time of day with invalid dayTicks should be 0, got %f", got)
	}
}
