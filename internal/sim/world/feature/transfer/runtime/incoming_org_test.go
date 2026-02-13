package runtime

import (
	"testing"

	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func TestUpsertIncomingOrg_CreatesAndMerges(t *testing.T) {
	orgs := map[string]*modelpkg.Organization{}
	in := &OrgTransfer{
		OrgID:       "ORG_001",
		Kind:        modelpkg.OrgCity,
		Name:        "River",
		CreatedTick: 12,
		MetaVersion: 2,
		Members: map[string]modelpkg.OrgRole{
			"A01": modelpkg.OrgLeader,
		},
	}
	org := UpsertIncomingOrg(orgs, in, "", "A99")
	if org == nil {
		t.Fatalf("expected org")
	}
	if org.Kind != modelpkg.OrgCity || org.Name != "River" {
		t.Fatalf("unexpected org header: %+v", org)
	}
	if org.Members["A01"] != modelpkg.OrgLeader {
		t.Fatalf("missing transferred member")
	}
	if org.Members["A99"] == "" {
		t.Fatalf("joining agent should be a member")
	}
}

func TestUpsertIncomingOrg_FallbackOrgID(t *testing.T) {
	orgs := map[string]*modelpkg.Organization{}
	org := UpsertIncomingOrg(orgs, nil, "ORG_FALLBACK", "A10")
	if org == nil || org.OrgID != "ORG_FALLBACK" {
		t.Fatalf("expected fallback org, got %+v", org)
	}
	if org.Members["A10"] == "" {
		t.Fatalf("expected fallback member")
	}
}
