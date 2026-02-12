package org

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

func TestMergeMeta(t *testing.T) {
	dst := Meta{
		OrgID:       "ORG1",
		Kind:        "GUILD",
		Name:        "Old",
		CreatedTick: 100,
		MetaVersion: 3,
		Members: map[string]string{
			"a1": "OWNER",
		},
	}
	src := Meta{
		OrgID:       "ORG1",
		Kind:        "CITY",
		Name:        "New",
		CreatedTick: 50,
		MetaVersion: 4,
		Members: map[string]string{
			"a2": "MEMBER",
		},
	}
	merged, ok := MergeMeta(dst, src)
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
	next, ok := MergeMeta(merged, stale)
	if ok {
		t.Fatalf("expected stale merge to be rejected")
	}
	if next.MetaVersion != merged.MetaVersion {
		t.Fatalf("stale merge mutated destination: %#v", next)
	}
}

func TestOwnerByAgent(t *testing.T) {
	owners := OwnerByAgent(map[string]Meta{
		"ORG2": {OrgID: "ORG2", Members: map[string]string{"a1": "MEMBER"}},
		"ORG1": {OrgID: "ORG1", Members: map[string]string{"a1": "OWNER", "a2": "MEMBER"}},
	})
	if owners["a1"] != "ORG1" || owners["a2"] != "ORG1" {
		t.Fatalf("unexpected owner map: %#v", owners)
	}
}

func TestMergeMetaMaps(t *testing.T) {
	existing := map[string]Meta{
		"ORG1": {
			OrgID:       "ORG1",
			Kind:        "GUILD",
			Name:        "One",
			CreatedTick: 10,
			MetaVersion: 2,
			Members: map[string]string{
				"a1": "OWNER",
			},
		},
	}
	incoming := map[string]Meta{
		"ORG1": {
			OrgID:       "ORG1",
			Kind:        "CITY",
			Name:        "One v3",
			CreatedTick: 10,
			MetaVersion: 3,
			Members: map[string]string{
				"a2": "MEMBER",
			},
		},
		"ORG2": {
			OrgID:       "ORG2",
			Kind:        "GUILD",
			Name:        "Two",
			CreatedTick: 20,
			MetaVersion: 1,
			Members: map[string]string{
				"b1": "OWNER",
			},
		},
	}
	merged := MergeMetaMaps(existing, incoming)
	if len(merged) != 2 {
		t.Fatalf("want 2 orgs, got %d", len(merged))
	}
	if merged["ORG1"].MetaVersion != 3 || merged["ORG1"].Members["a2"] != "MEMBER" {
		t.Fatalf("org1 not upgraded correctly: %#v", merged["ORG1"])
	}
	if merged["ORG2"].OrgID != "ORG2" {
		t.Fatalf("org2 missing: %#v", merged)
	}
}
