package claims

import "testing"

func TestValidateUpgradeRadius(t *testing.T) {
	ok, code, _ := ValidateUpgradeRadius(32, 63)
	if ok || code != "E_BAD_REQUEST" {
		t.Fatalf("expected invalid radius guard")
	}

	ok, code, _ = ValidateUpgradeRadius(64, 64)
	if ok || code != "E_BAD_REQUEST" {
		t.Fatalf("expected non-increase guard")
	}

	ok, code, _ = ValidateUpgradeRadius(32, 64)
	if !ok || code != "" {
		t.Fatalf("expected valid upgrade")
	}
}

func TestUpgradeCost(t *testing.T) {
	cost := UpgradeCost(32, 64)
	if cost["BATTERY"] != 1 || cost["CRYSTAL_SHARD"] != 2 {
		t.Fatalf("unexpected 32->64 cost: %#v", cost)
	}

	cost = UpgradeCost(64, 128)
	if cost["BATTERY"] != 2 || cost["CRYSTAL_SHARD"] != 4 {
		t.Fatalf("unexpected 64->128 cost: %#v", cost)
	}

	cost = UpgradeCost(32, 128)
	if cost["BATTERY"] != 3 || cost["CRYSTAL_SHARD"] != 6 {
		t.Fatalf("unexpected 32->128 cost: %#v", cost)
	}
}

func TestApplyPolicyFlags(t *testing.T) {
	base := Flags{AllowBuild: true, AllowBreak: false, AllowDamage: true, AllowTrade: false}
	next := ApplyPolicyFlags(base, map[string]bool{"allow_break": true, "allow_trade": true})
	if !next.AllowBuild || !next.AllowBreak || !next.AllowDamage || !next.AllowTrade {
		t.Fatalf("unexpected flags: %#v", next)
	}
}
