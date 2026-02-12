package runtime

import (
	"testing"

	corepkg "voxelcraft.ai/internal/sim/world/feature/contracts/core"
)

func TestBuildTerminalSummariesFiltersByTerminal(t *testing.T) {
	summaries := BuildTerminalSummaries([3]int{1, 0, 2}, []SummaryContract{
		{ContractID: "C1", TerminalPos: [3]int{1, 0, 2}, State: "OPEN", Kind: "GATHER", Poster: "A1", Acceptor: "A2", DeadlineTick: 100},
		{ContractID: "C2", TerminalPos: [3]int{9, 0, 9}, State: "OPEN", Kind: "BUILD", Poster: "A1", Acceptor: "A3", DeadlineTick: 100},
	})
	if len(summaries) != 1 {
		t.Fatalf("expected one summary, got %d", len(summaries))
	}
	if summaries[0]["contract_id"] != "C1" {
		t.Fatalf("unexpected contract id: %#v", summaries[0]["contract_id"])
	}
}

func TestPayoutAgent(t *testing.T) {
	if got := PayoutAgent("P", "A", corepkg.PayoutPoster); got != "P" {
		t.Fatalf("expected poster, got %q", got)
	}
	if got := PayoutAgent("P", "A", corepkg.PayoutAcceptor); got != "A" {
		t.Fatalf("expected acceptor, got %q", got)
	}
}

func TestApplyTerminalTransfer(t *testing.T) {
	inv := map[string]int{"IRON_INGOT": 5}
	unreserved := 0
	agentInv := map[string]map[string]int{}
	owed := map[string]map[string]int{}

	ApplyTerminalTransfer(
		inv,
		map[string]int{"IRON_INGOT": 3},
		"A1",
		true,
		func(item string, n int) {
			if item == "IRON_INGOT" {
				unreserved += n
			}
		},
		func(agentID, item string, n int) bool {
			if agentID == "A1" {
				if agentInv[agentID] == nil {
					agentInv[agentID] = map[string]int{}
				}
				agentInv[agentID][item] += n
				return true
			}
			return false
		},
		func(agentID, item string, n int) {
			if owed[agentID] == nil {
				owed[agentID] = map[string]int{}
			}
			owed[agentID][item] += n
		},
	)
	if unreserved != 3 {
		t.Fatalf("expected unreserve=3 got=%d", unreserved)
	}
	if inv["IRON_INGOT"] != 2 {
		t.Fatalf("unexpected terminal inventory: %d", inv["IRON_INGOT"])
	}
	if agentInv["A1"]["IRON_INGOT"] != 3 {
		t.Fatalf("unexpected agent inventory transfer: %d", agentInv["A1"]["IRON_INGOT"])
	}
	if len(owed) != 0 {
		t.Fatalf("expected no owed items, got %#v", owed)
	}
}

func TestConsumeAndPayout(t *testing.T) {
	inv := map[string]int{"IRON_INGOT": 6, "COPPER_INGOT": 4}
	poster := map[string]int{}
	receiver := map[string]int{}
	ConsumeRequirementsToPoster(inv, map[string]int{"IRON_INGOT": 2}, func(item string, n int) {
		poster[item] += n
	})
	if inv["IRON_INGOT"] != 4 || poster["IRON_INGOT"] != 2 {
		t.Fatalf("unexpected requirement consume state inv=%v poster=%v", inv, poster)
	}
	PayoutItems(inv, map[string]int{"COPPER_INGOT": 3}, nil, func(item string, n int) {
		receiver[item] += n
	})
	if inv["COPPER_INGOT"] != 1 || receiver["COPPER_INGOT"] != 3 {
		t.Fatalf("unexpected payout state inv=%v receiver=%v", inv, receiver)
	}
}
