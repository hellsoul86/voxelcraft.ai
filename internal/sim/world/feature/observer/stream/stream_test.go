package stream

import "testing"

func TestClampInt(t *testing.T) {
	if got := ClampInt(100, 1, 10, 5); got != 10 {
		t.Fatalf("ClampInt=%d, want 10", got)
	}
}

func TestComputeWantedChunksNotEmpty(t *testing.T) {
	out := ComputeWantedChunks([]ChunkKey{{CX: 0, CZ: 0}}, 1, 32)
	if len(out) == 0 {
		t.Fatalf("expected non-empty wanted chunks")
	}
}
