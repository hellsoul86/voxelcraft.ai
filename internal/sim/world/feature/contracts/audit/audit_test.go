package audit

import (
	"testing"

	corepkg "voxelcraft.ai/internal/sim/world/feature/contracts/core"
)

func TestBuildPostAuditFields(t *testing.T) {
	fields := BuildPostAuditFields("C000001", "CONTRACT_TERMINAL@1,0,1", "BUILD", nil, nil, nil, 1200, "workshop_pad", [3]int{1, 0, 1}, 0)
	if fields["contract_id"] != "C000001" || fields["kind"] != "BUILD" {
		t.Fatalf("unexpected post audit fields: %#v", fields)
	}
}

func TestBuildAcceptAuditFields(t *testing.T) {
	fields := BuildAcceptAuditFields("C000001", "CONTRACT_TERMINAL@1,0,1", "DELIVER", "A1", "A2", nil)
	if fields["poster"] != "A1" || fields["acceptor"] != "A2" {
		t.Fatalf("unexpected accept audit fields: %#v", fields)
	}
}

func TestBuildSubmitAuditFields(t *testing.T) {
	fields := BuildSubmitAuditFields("C000001", "CONTRACT_TERMINAL@1,0,1", "GATHER", "A1", "A2")
	if fields["terminal_id"] != "CONTRACT_TERMINAL@1,0,1" {
		t.Fatalf("unexpected submit audit fields: %#v", fields)
	}
}

func TestBuildTickAudit(t *testing.T) {
	out := BuildTickAudit(TickAuditInput{
		Decision:    corepkg.DecisionCompleteDeliver,
		AuditReason: "AUTO_COMPLETE",
		ContractID:  "C000001",
		Kind:        "DELIVER",
		Poster:      "A1",
		Acceptor:    "A2",
		State:       "COMPLETED",
	})
	if !out.ShouldAudit || out.EventType != "CONTRACT_COMPLETE" || !out.GrantTradeCredit {
		t.Fatalf("unexpected complete deliver audit output: %#v", out)
	}

	out = BuildTickAudit(TickAuditInput{
		Decision:    corepkg.DecisionTimeoutAccepted,
		AuditReason: "CONTRACT_TIMEOUT",
		ContractID:  "C000002",
	})
	if !out.ShouldAudit || out.EventType != "CONTRACT_FAIL" || out.Actor != "WORLD" {
		t.Fatalf("unexpected timeout audit output: %#v", out)
	}
}
