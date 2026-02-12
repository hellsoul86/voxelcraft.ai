package permissions

import (
	"testing"

	claimspkg "voxelcraft.ai/internal/sim/world/feature/governance/claims"
)

func TestForLand(t *testing.T) {
	flags := claimspkg.Flags{
		AllowBuild:  false,
		AllowBreak:  false,
		AllowDamage: true,
		AllowTrade:  false,
	}
	member := ForLand(true, 0, flags)
	if !member.CanBuild || !member.CanBreak || !member.CanTrade || !member.CanDamage {
		t.Fatalf("unexpected member perms: %#v", member)
	}
	visitor := ForLand(false, 0, flags)
	if visitor.CanBuild || visitor.CanBreak || visitor.CanTrade || !visitor.CanDamage {
		t.Fatalf("unexpected visitor perms: %#v", visitor)
	}
	late := ForLand(false, 2, flags)
	if !late.CanBuild || !late.CanBreak || !late.CanTrade || late.CanDamage {
		t.Fatalf("unexpected degraded perms: %#v", late)
	}
}
