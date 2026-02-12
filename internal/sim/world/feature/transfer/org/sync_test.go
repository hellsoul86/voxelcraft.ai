package org

import "testing"

func TestNormalizeStates(t *testing.T) {
	in := []State{
		{OrgID: "ORG1", Kind: "GUILD", Name: "Foundry", CreatedTick: 1, MetaVersion: 2, Members: map[string]string{"A1": "LEADER", "": "MEMBER"}},
	}
	out := NormalizeStates(in)
	if len(out) != 1 {
		t.Fatalf("expected one org state")
	}
	if _, ok := out[0].Members[""]; ok {
		t.Fatalf("expected empty member id to be removed")
	}
}

func TestMergeStates(t *testing.T) {
	existing := []State{
		{
			OrgID:       "ORG1",
			Kind:        "GUILD",
			Name:        "Foundry",
			CreatedTick: 1,
			MetaVersion: 2,
			Members:     map[string]string{"A1": "LEADER"},
		},
	}
	incoming := []State{
		{
			OrgID:       "ORG1",
			Kind:        "GUILD",
			Name:        "Foundry",
			CreatedTick: 1,
			MetaVersion: 3,
			Members:     map[string]string{"A1": "LEADER", "A2": "MEMBER"},
		},
	}
	merged, owners := MergeStates(existing, incoming)
	if len(merged) != 1 {
		t.Fatalf("expected one merged org")
	}
	if len(merged[0].Members) != 2 {
		t.Fatalf("expected incoming members applied: %#v", merged[0].Members)
	}
	if owners["A2"] != "ORG1" {
		t.Fatalf("expected owner map to include new member")
	}
}
