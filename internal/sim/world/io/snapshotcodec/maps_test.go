package snapshotcodec

import "testing"

func TestPositiveNestedMap(t *testing.T) {
	got := PositiveNestedMap(map[string]map[string]int{
		"a": {"X": 1, "Y": 0},
		"b": {"Z": -1},
		"":  {"K": 1},
	})
	if len(got) != 1 {
		t.Fatalf("PositiveNestedMap len=%d, want 1", len(got))
	}
	if got["a"]["X"] != 1 {
		t.Fatalf("missing preserved positive value")
	}
}
