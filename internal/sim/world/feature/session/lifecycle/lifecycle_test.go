package lifecycle

import "testing"

func TestNewAgentID(t *testing.T) {
	if got := NewAgentID(17); got != "A17" {
		t.Fatalf("NewAgentID = %q, want A17", got)
	}
}

func TestSpawnSeed(t *testing.T) {
	x, z := SpawnSeed(3)
	if x != 6 || z != -6 {
		t.Fatalf("SpawnSeed = (%d,%d), want (6,-6)", x, z)
	}
}

func TestNewResumeToken(t *testing.T) {
	got := NewResumeToken("OVERWORLD", 123)
	want := "resume_OVERWORLD_123"
	if got != want {
		t.Fatalf("NewResumeToken = %q, want %q", got, want)
	}
}
