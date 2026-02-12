package world

import "testing"

func TestOrgTreasuryFor_MigratesLegacyOnce(t *testing.T) {
	org := &Organization{Treasury: map[string]int{"IRON_INGOT": 3}}

	overworld := org.TreasuryFor("OVERWORLD")
	if got := overworld["IRON_INGOT"]; got != 3 {
		t.Fatalf("overworld seed mismatch: got %d want 3", got)
	}
	overworld["PLANK"] = 2

	mine := org.TreasuryFor("MINE_L1")
	if len(mine) != 0 {
		t.Fatalf("new world treasury should start empty, got %v", mine)
	}
	if got := org.TreasuryByWorld["OVERWORLD"]["PLANK"]; got != 2 {
		t.Fatalf("overworld treasury should remain isolated: got %d want 2", got)
	}
}

func TestOrgTreasuryFor_UsesGlobalKeyForEmptyWorldID(t *testing.T) {
	org := &Organization{Treasury: map[string]int{"COAL": 1}}
	got := org.TreasuryFor("")
	if got == nil {
		t.Fatalf("expected treasury map")
	}
	if _, ok := org.TreasuryByWorld["GLOBAL"]; !ok {
		t.Fatalf("expected GLOBAL treasury bucket")
	}
}
