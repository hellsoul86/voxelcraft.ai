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

func TestCanWithdrawContainer(t *testing.T) {
	if !CanWithdrawContainer(false, false, 0) {
		t.Fatalf("wild container should allow withdraw")
	}
	if CanWithdrawContainer(true, false, 0) {
		t.Fatalf("visitor should not withdraw in protected land")
	}
	if !CanWithdrawContainer(true, true, 0) {
		t.Fatalf("member should withdraw")
	}
	if !CanWithdrawContainer(true, false, 2) {
		t.Fatalf("visitor should withdraw after protection downgrade")
	}
}
