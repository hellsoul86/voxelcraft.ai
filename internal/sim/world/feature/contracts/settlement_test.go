package contracts

import "testing"

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
