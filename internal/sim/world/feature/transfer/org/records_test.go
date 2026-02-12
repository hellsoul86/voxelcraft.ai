package org

import "testing"

func TestOrgRecordsRoundtrip(t *testing.T) {
	records := []Record{
		{
			OrgID:       "ORG2",
			Kind:        "CITY",
			Name:        "Two",
			CreatedTick: 20,
			MetaVersion: 2,
			Members: map[string]string{
				"a2": "MEMBER",
				"":   "OWNER",
			},
		},
		{
			OrgID:       "ORG1",
			Kind:        "GUILD",
			Name:        "One",
			CreatedTick: 10,
			MetaVersion: 1,
			Members: map[string]string{
				"a1": "OWNER",
			},
		},
	}
	meta := MetaMapFromRecords(records)
	out := SortedRecordsFromMeta(meta)
	if len(out) != 2 {
		t.Fatalf("want 2 org records, got %d", len(out))
	}
	if out[0].OrgID != "ORG1" || out[1].OrgID != "ORG2" {
		t.Fatalf("unexpected sort order: %+v", out)
	}
	if out[1].Members["a2"] != "MEMBER" {
		t.Fatalf("member normalization mismatch: %+v", out[1].Members)
	}
}

func TestMergeRecords(t *testing.T) {
	existing := []Record{
		{
			OrgID:       "ORG1",
			Kind:        "GUILD",
			Name:        "One",
			CreatedTick: 10,
			MetaVersion: 1,
			Members:     map[string]string{"a1": "OWNER"},
		},
	}
	incoming := []Record{
		{
			OrgID:       "ORG1",
			Kind:        "CITY",
			Name:        "One v2",
			CreatedTick: 10,
			MetaVersion: 2,
			Members:     map[string]string{"a2": "MEMBER"},
		},
	}
	merged, owner := MergeRecords(existing, incoming)
	if len(merged) != 1 || merged[0].MetaVersion != 2 {
		t.Fatalf("unexpected merged: %+v", merged)
	}
	if owner["a2"] != "ORG1" {
		t.Fatalf("unexpected owner map: %+v", owner)
	}
}
