package core

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

func TestDecideTick(t *testing.T) {
	if got := DecideTick(TickInput{State: "OPEN", HasTerminal: false}); got != DecisionFailMissingTarget {
		t.Fatalf("expected missing terminal fail, got %s", got)
	}
	if got := DecideTick(TickInput{State: "OPEN", HasTerminal: true, NowTick: 11, DeadlineTick: 10}); got != DecisionExpireOpen {
		t.Fatalf("expected open expire, got %s", got)
	}
	if got := DecideTick(TickInput{State: "ACCEPTED", HasTerminal: true, NowTick: 11, DeadlineTick: 10}); got != DecisionTimeoutAccepted {
		t.Fatalf("expected accepted timeout, got %s", got)
	}
	if got := DecideTick(TickInput{State: "ACCEPTED", Kind: "DELIVER", HasTerminal: true, RequirementsOK: true}); got != DecisionCompleteDeliver {
		t.Fatalf("expected complete deliver, got %s", got)
	}
	if got := DecideTick(TickInput{State: "ACCEPTED", Kind: "BUILD", HasTerminal: true, BuildPlaced: true, BuildStable: true}); got != DecisionCompleteBuild {
		t.Fatalf("expected complete build, got %s", got)
	}
	if got := DecideTick(TickInput{State: "ACCEPTED", Kind: "BUILD", HasTerminal: true, BuildPlaced: true, BuildStable: false}); got != DecisionNoop {
		t.Fatalf("expected noop, got %s", got)
	}
}

func TestPlanSettlement(t *testing.T) {
	if p := PlanSettlement(DecisionFailMissingTarget); !p.MarkFailed || p.MarkCompleted {
		t.Fatalf("unexpected missing target plan: %+v", p)
	}
	if p := PlanSettlement(DecisionExpireOpen); !p.MarkFailed || p.RewardTo != PayoutPoster {
		t.Fatalf("unexpected expire plan: %+v", p)
	}
	if p := PlanSettlement(DecisionTimeoutAccepted); !p.MarkFailed || p.DepositTo != PayoutPoster || !p.PenalizeAcceptorLaw {
		t.Fatalf("unexpected timeout accepted plan: %+v", p)
	}
	if p := PlanSettlement(DecisionCompleteDeliver); !p.MarkCompleted || !p.ConsumeRequirements || p.RewardTo != PayoutAcceptor {
		t.Fatalf("unexpected complete deliver plan: %+v", p)
	}
	if p := PlanSettlement(DecisionCompleteBuild); !p.MarkCompleted || p.DepositTo != PayoutAcceptor {
		t.Fatalf("unexpected complete build plan: %+v", p)
	}
}
