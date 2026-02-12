package resume

import "testing"

func TestSortedIDs(t *testing.T) {
	got := SortedIDs(map[string]int{"b": 1, "a": 2})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected sorted ids: %#v", got)
	}
}

func TestFindResumeAgentID(t *testing.T) {
	id := FindResumeAgentID([]Candidate{
		{ID: "A2", ResumeToken: "t2"},
		{ID: "A1", ResumeToken: "t1"},
	}, "t1")
	if id != "A1" {
		t.Fatalf("expected A1, got %q", id)
	}
}
