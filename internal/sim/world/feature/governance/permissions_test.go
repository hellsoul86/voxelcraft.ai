package governance

import "testing"

func TestPermissionsForLand(t *testing.T) {
	flags := ClaimFlags{
		AllowBuild:  false,
		AllowBreak:  false,
		AllowDamage: true,
		AllowTrade:  false,
	}
	member := PermissionsForLand(true, 0, flags)
	if !member.CanBuild || !member.CanBreak || !member.CanTrade || !member.CanDamage {
		t.Fatalf("unexpected member perms: %#v", member)
	}
	visitor := PermissionsForLand(false, 0, flags)
	if visitor.CanBuild || visitor.CanBreak || visitor.CanTrade || !visitor.CanDamage {
		t.Fatalf("unexpected visitor perms: %#v", visitor)
	}
	late := PermissionsForLand(false, 2, flags)
	if !late.CanBuild || !late.CanBreak || !late.CanTrade || late.CanDamage {
		t.Fatalf("unexpected degraded perms: %#v", late)
	}
}
