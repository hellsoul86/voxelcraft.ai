package orgs

import "testing"

func TestNormalizeOrgKind(t *testing.T) {
	if got := NormalizeOrgKind(" guild "); got != KindGuild {
		t.Fatalf("expected guild, got %q", got)
	}
	if got := NormalizeOrgKind("CITY"); got != KindCity {
		t.Fatalf("expected city, got %q", got)
	}
	if got := NormalizeOrgKind("team"); got != "" {
		t.Fatalf("expected unknown kind empty, got %q", got)
	}
}

func TestValidateOrgName(t *testing.T) {
	if ValidateOrgName("") {
		t.Fatalf("empty org name should be invalid")
	}
	if !ValidateOrgName("Foundry") {
		t.Fatalf("simple org name should be valid")
	}
}

func TestSelectNextLeader(t *testing.T) {
	got := SelectNextLeader([]string{"B2", "A9", "A1"})
	if got != "A1" {
		t.Fatalf("expected lexicographically smallest member, got %q", got)
	}
}
