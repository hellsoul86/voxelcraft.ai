package runtime

import "testing"

func TestMoveTolerance(t *testing.T) {
	if got := MoveTolerance(0); got != 1 {
		t.Fatalf("MoveTolerance(0)=%d", got)
	}
	if got := MoveTolerance(1.2); got != 2 {
		t.Fatalf("MoveTolerance(1.2)=%d", got)
	}
	if got := MoveTolerance(3.0); got != 3 {
		t.Fatalf("MoveTolerance(3.0)=%d", got)
	}
}

func TestSkipRules(t *testing.T) {
	if !ShouldSkipStorm("STORM", 3) {
		t.Fatalf("storm odd tick should skip")
	}
	if ShouldSkipStorm("STORM", 2) {
		t.Fatalf("storm even tick should not skip")
	}
	if !ShouldSkipFlood("FLOOD_WARNING", 10, 4, 10, Pos{X: 1, Z: 1}, Pos{X: 0, Z: 0}) {
		t.Fatalf("flood hazard should skip")
	}
	if ShouldSkipFlood("FLOOD_WARNING", 10, 11, 10, Pos{X: 1, Z: 1}, Pos{X: 0, Z: 0}) {
		t.Fatalf("expired flood should not skip")
	}
}

func TestPrimarySecondarySteps(t *testing.T) {
	cur := Pos{X: 0, Z: 0}
	next := PrimaryStep(cur, 3, 1, true)
	if next.X != 1 || next.Z != 0 {
		t.Fatalf("primary step mismatch: %+v", next)
	}
	alt := SecondaryStep(cur, 3, 1, true)
	if alt.X != 0 || alt.Z != 1 {
		t.Fatalf("secondary step mismatch: %+v", alt)
	}
}
