package rules

import "testing"

func TestInWindow(t *testing.T) {
	// Simple window (no wrap).
	if !InWindow(0.05, 0.0, 0.1) {
		t.Fatalf("expected inside window")
	}
	if InWindow(0.2, 0.0, 0.1) {
		t.Fatalf("expected outside window")
	}

	// Wrap-around window.
	if !InWindow(0.95, 0.9, 0.1) {
		t.Fatalf("expected inside wrap-around window at high end")
	}
	if !InWindow(0.05, 0.9, 0.1) {
		t.Fatalf("expected inside wrap-around window at low end")
	}
	if InWindow(0.5, 0.9, 0.1) {
		t.Fatalf("expected outside wrap-around window")
	}
}

func TestCanActionWithCurfew(t *testing.T) {
	if CanActionWithCurfew(false, false, 0.0, 0.0, 0.1) {
		t.Fatalf("base denied should deny")
	}
	if !CanActionWithCurfew(true, false, 0.0, 0.0, 0.1) {
		t.Fatalf("curfew disabled should allow")
	}
	// Curfew enabled: deny inside, allow outside.
	if CanActionWithCurfew(true, true, 0.05, 0.0, 0.1) {
		t.Fatalf("expected denied inside curfew window")
	}
	if !CanActionWithCurfew(true, true, 0.2, 0.0, 0.1) {
		t.Fatalf("expected allowed outside curfew window")
	}
}
