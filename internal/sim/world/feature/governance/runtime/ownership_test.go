package runtime

import (
	"testing"

	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func TestLandAtAndMembership(t *testing.T) {
	claims := map[string]*modelpkg.LandClaim{
		"L1": {
			LandID: "L1",
			Anchor: modelpkg.Vec3i{X: 10, Y: 0, Z: 10},
			Radius: 4,
			Owner:  "ORG01",
			Members: map[string]bool{
				"a2": true,
			},
		},
	}
	orgs := map[string]*modelpkg.Organization{
		"ORG01": {
			OrgID: "ORG01",
			Members: map[string]modelpkg.OrgRole{
				"a1": modelpkg.OrgMember,
			},
		},
	}

	land := LandAt(claims, modelpkg.Vec3i{X: 12, Y: 0, Z: 11})
	if land == nil || land.LandID != "L1" {
		t.Fatalf("expected land L1, got %+v", land)
	}
	if !IsLandMember(orgs, "a1", land) {
		t.Fatalf("expected org member to be treated as land member")
	}
	if !IsLandMember(orgs, "a2", land) {
		t.Fatalf("expected explicit land member")
	}
	if IsLandMember(orgs, "ax", land) {
		t.Fatalf("unexpected member")
	}
}

func TestSortedClaimIDs(t *testing.T) {
	claims := map[string]*modelpkg.LandClaim{
		"B": {LandID: "B"},
		"A": {LandID: "A"},
		"C": {LandID: "C"},
	}
	ids := SortedClaimIDs(claims)
	if len(ids) != 3 || ids[0] != "A" || ids[1] != "B" || ids[2] != "C" {
		t.Fatalf("unexpected ids order: %+v", ids)
	}
}
