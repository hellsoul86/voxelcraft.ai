package spawns

import "testing"

func TestSquareCount(t *testing.T) {
	got := Square(Pos{X: 10, Y: 0, Z: 10}, 2)
	if len(got) != 25 {
		t.Fatalf("Square len=%d, want 25", len(got))
	}
}

func TestDiamondCount(t *testing.T) {
	got := Diamond(Pos{X: 0, Y: 0, Z: 0}, 2)
	// 1 + 4 + 8
	if len(got) != 13 {
		t.Fatalf("Diamond len=%d, want 13", len(got))
	}
}

func TestRingSquareCount(t *testing.T) {
	got := RingSquare(Pos{X: 0, Y: 0, Z: 0}, 2)
	if len(got) != 16 {
		t.Fatalf("RingSquare len=%d, want 16", len(got))
	}
}
