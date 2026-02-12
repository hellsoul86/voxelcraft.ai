package resources

import "testing"

func TestComputeResourceDensity(t *testing.T) {
	targets := []string{"COAL_ORE", "IRON_ORE", "CRYSTAL_ORE"}
	index := map[string]uint16{"AIR": 0, "COAL_ORE": 1, "IRON_ORE": 2, "CRYSTAL_ORE": 3}
	chunks := [][]uint16{{0, 0, 1, 1, 2, 0, 2, 0}}

	d := ComputeResourceDensity(targets, index, chunks)
	if got := d["COAL_ORE"]; got != 0.25 {
		t.Fatalf("coal density mismatch: got %f want 0.25", got)
	}
	if got := d["IRON_ORE"]; got != 0.25 {
		t.Fatalf("iron density mismatch: got %f want 0.25", got)
	}
	if got := d["CRYSTAL_ORE"]; got != 0 {
		t.Fatalf("crystal density mismatch: got %f want 0", got)
	}
}
