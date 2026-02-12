package runtime

import "testing"

func TestHungerAfterTick(t *testing.T) {
	if got := HungerAfterTick(10, false); got != 9 {
		t.Fatalf("expected 9, got %d", got)
	}
	if got := HungerAfterTick(1, true); got != 0 {
		t.Fatalf("expected clamped 0, got %d", got)
	}
}

func TestIsNight(t *testing.T) {
	if !IsNight(0.1) || !IsNight(0.9) || IsNight(0.5) {
		t.Fatalf("night/day classification mismatch")
	}
}

func TestStaminaRecovery(t *testing.T) {
	if got := StaminaRecovery("CLEAR", 10, "", false); got != 2 {
		t.Fatalf("expected 2, got %d", got)
	}
	if got := StaminaRecovery("COLD", 10, "", false); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	if got := StaminaRecovery("CLEAR", 0, "", false); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
	if got := StaminaRecovery("CLEAR", 10, "BLIGHT_ZONE", true); got != 0 {
		t.Fatalf("expected 0 in blight, got %d", got)
	}
}
