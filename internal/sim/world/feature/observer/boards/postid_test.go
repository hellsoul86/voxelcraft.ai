package boards

import (
	"testing"

	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func TestNewPostID(t *testing.T) {
	if got := NewPostID(42); got != "P000042" {
		t.Fatalf("NewPostID mismatch: got %q want P000042", got)
	}
}

func TestEnsureAndRemoveBoard(t *testing.T) {
	pos := modelpkg.Vec3i{X: 3, Y: 0, Z: 4}
	boards := map[string]*Board{}

	b := EnsureBoard(boards, pos)
	if b == nil {
		t.Fatalf("expected board instance")
	}
	if b.BoardID == "" {
		t.Fatalf("expected non-empty board id")
	}
	if got := boards[BoardIDAt(pos)]; got == nil {
		t.Fatalf("expected board stored by canonical id")
	}

	RemoveBoard(boards, pos)
	if got := boards[BoardIDAt(pos)]; got != nil {
		t.Fatalf("expected board removed")
	}
}
