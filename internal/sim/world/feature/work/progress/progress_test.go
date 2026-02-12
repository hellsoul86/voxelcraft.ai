package progress

import "testing"

func TestTimedProgress(t *testing.T) {
	if got := TimedProgress(2, 5); got != 0.4 {
		t.Fatalf("expected 0.4, got %v", got)
	}
	if got := TimedProgress(10, 5); got != 1 {
		t.Fatalf("expected clamped 1, got %v", got)
	}
	if got := TimedProgress(1, 0); got != 0 {
		t.Fatalf("expected 0 for invalid total, got %v", got)
	}
}

func TestBlueprintProgress(t *testing.T) {
	if got := BlueprintProgress(1, 4); got != 0.25 {
		t.Fatalf("expected 0.25, got %v", got)
	}
}

func TestMineProgressUsesBestTool(t *testing.T) {
	inv := map[string]int{"IRON_PICKAXE": 1}
	if got := MineProgress(2, "STONE", inv); got != 0.5 {
		t.Fatalf("expected 0.5 progress with iron pickaxe on STONE, got %v", got)
	}
}
