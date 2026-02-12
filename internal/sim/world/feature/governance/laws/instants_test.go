package laws

import "testing"

func TestValidateProposeInput(t *testing.T) {
	ok, code, _ := ValidateProposeInput(false, "LAND_1", "MARKET_TAX", map[string]interface{}{"tax_rate": 0.1})
	if ok || code != "E_NO_PERMISSION" {
		t.Fatalf("expected laws disabled guard")
	}
	ok, code, _ = ValidateProposeInput(true, "", "MARKET_TAX", map[string]interface{}{})
	if ok || code != "E_BAD_REQUEST" {
		t.Fatalf("expected missing ids guard")
	}
}

func TestBuildLawTimeline(t *testing.T) {
	tl := BuildLawTimeline(100, 3000, 3000)
	if tl.NoticeEnds != 3100 || tl.VoteEnds != 6100 {
		t.Fatalf("unexpected timeline: %#v", tl)
	}
}

func TestValidateVoteInput(t *testing.T) {
	ok, code, _ := ValidateVoteInput(true, "", "YES")
	if ok || code != "E_BAD_REQUEST" {
		t.Fatalf("expected missing vote fields guard")
	}
	ok, code, _ = ValidateVoteInput(true, "LAW_1", "YES")
	if !ok || code != "" {
		t.Fatalf("expected valid vote input")
	}
}
