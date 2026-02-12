package chunks

import "testing"

func TestComputeWantedChunks(t *testing.T) {
	out := ComputeWantedChunks([]Key{{CX: 0, CZ: 0}}, 1, 4)
	if len(out) != 4 {
		t.Fatalf("expected clipped 4 chunks, got %d", len(out))
	}
	if out[0] != (Key{CX: 0, CZ: 0}) {
		t.Fatalf("closest chunk should be center, got %+v", out[0])
	}
}

func TestClampAndCeil(t *testing.T) {
	if got := ClampInt(0, 1, 5, 3); got != 3 {
		t.Fatalf("ClampInt default failed: %d", got)
	}
	if got := Ceil(1.1); got != 2 {
		t.Fatalf("Ceil failed: %v", got)
	}
}
