package claims

import "testing"

func TestUpgradeOverlaps(t *testing.T) {
	zones := []Zone{
		{LandID: "LAND_A", AnchorX: 0, AnchorZ: 0, Radius: 32},
		{LandID: "LAND_B", AnchorX: 100, AnchorZ: 100, Radius: 32},
	}
	if !UpgradeOverlaps(50, 50, 64, "LAND_A", zones) {
		t.Fatalf("expected overlap with LAND_B at upgrade radius")
	}
	if UpgradeOverlaps(200, 200, 32, "LAND_A", zones) {
		t.Fatalf("expected no overlap far from other claims")
	}
}
