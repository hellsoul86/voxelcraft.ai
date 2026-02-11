package transfer

import "testing"

func TestNormalizeMembers(t *testing.T) {
	got := NormalizeMembers(map[string]string{
		"a1": "OWNER",
		"":   "MEMBER",
		"a2": "",
	})
	if len(got) != 1 || got["a1"] != "OWNER" {
		t.Fatalf("unexpected members: %#v", got)
	}
}

func TestMergeOrgMeta(t *testing.T) {
	dst := OrgMeta{
		OrgID:       "ORG1",
		Kind:        "GUILD",
		Name:        "Old",
		CreatedTick: 100,
		MetaVersion: 3,
		Members: map[string]string{
			"a1": "OWNER",
		},
	}
	src := OrgMeta{
		OrgID:       "ORG1",
		Kind:        "CITY",
		Name:        "New",
		CreatedTick: 50,
		MetaVersion: 4,
		Members: map[string]string{
			"a2": "MEMBER",
		},
	}
	merged, ok := MergeOrgMeta(dst, src)
	if !ok {
		t.Fatalf("expected merge to be accepted")
	}
	if merged.Kind != "CITY" || merged.Name != "New" || merged.CreatedTick != 50 || merged.MetaVersion != 4 {
		t.Fatalf("unexpected merged org: %#v", merged)
	}
	if len(merged.Members) != 1 || merged.Members["a2"] != "MEMBER" {
		t.Fatalf("unexpected merged members: %#v", merged.Members)
	}

	stale := src
	stale.MetaVersion = 2
	next, ok := MergeOrgMeta(merged, stale)
	if ok {
		t.Fatalf("expected stale merge to be rejected")
	}
	if next.MetaVersion != merged.MetaVersion {
		t.Fatalf("stale merge mutated destination: %#v", next)
	}
}

func TestOwnerByAgent(t *testing.T) {
	owners := OwnerByAgent(map[string]OrgMeta{
		"ORG2": {OrgID: "ORG2", Members: map[string]string{"a1": "MEMBER"}},
		"ORG1": {OrgID: "ORG1", Members: map[string]string{"a1": "OWNER", "a2": "MEMBER"}},
	})
	if owners["a1"] != "ORG1" || owners["a2"] != "ORG1" {
		t.Fatalf("unexpected owner map: %#v", owners)
	}
}
