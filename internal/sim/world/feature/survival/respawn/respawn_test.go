package respawn

import "testing"

func TestComputeInventoryLoss(t *testing.T) {
	inv := map[string]int{"PLANK": 10, "COAL": 1}
	lost := ComputeInventoryLoss(inv)
	if lost["PLANK"] != 3 {
		t.Fatalf("expected PLANK loss 3, got %d", lost["PLANK"])
	}
	if len(lost) == 0 {
		t.Fatalf("expected at least one lost item")
	}
}

func TestAgentNumber(t *testing.T) {
	if got := AgentNumber("A17"); got != 17 {
		t.Fatalf("unexpected agent number: %d", got)
	}
	if got := AgentNumber("BAD"); got != 0 {
		t.Fatalf("unexpected invalid parsing result: %d", got)
	}
}
