package contracts

import "testing"

func TestNormalizeKind(t *testing.T) {
	if got := NormalizeKind(" gather "); got != "GATHER" {
		t.Fatalf("expected GATHER, got %q", got)
	}
	if got := NormalizeKind("foo"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestBuildSummaries(t *testing.T) {
	got := BuildSummaries([]SummaryInput{
		{ContractID: "C2", State: "OPEN", Kind: "GATHER", Poster: "A2", Acceptor: "", DeadlineTick: 20},
		{ContractID: "C1", State: "ACCEPTED", Kind: "BUILD", Poster: "A1", Acceptor: "A3", DeadlineTick: 10},
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(got))
	}
	if got[0]["contract_id"] != "C1" || got[1]["contract_id"] != "C2" {
		t.Fatalf("expected sorted by id, got %#v", got)
	}
}
