package observerprogress

import "testing"

func TestFollowProgress(t *testing.T) {
	cur := Vec3{X: 0, Y: 0, Z: 0}
	target := Vec3{X: 3, Y: 0, Z: 0}
	prog, eta := FollowProgress(cur, target, 1.0)
	if prog != 0 {
		t.Fatalf("progress: got %v want 0", prog)
	}
	if eta != 2 {
		t.Fatalf("eta: got %d want 2", eta)
	}
}

func TestMoveProgress(t *testing.T) {
	start := Vec3{X: 0, Y: 0, Z: 0}
	cur := Vec3{X: 2, Y: 0, Z: 0}
	target := Vec3{X: 4, Y: 0, Z: 0}
	prog, eta := MoveProgress(start, cur, target, 1.0)
	if prog != 0.6666666666666666 {
		t.Fatalf("progress: got %.16f", prog)
	}
	if eta != 1 {
		t.Fatalf("eta: got %d want 1", eta)
	}
}
