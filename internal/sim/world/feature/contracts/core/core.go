package core

import (
	"sort"
	"strings"
)

type SummaryInput struct {
	ContractID   string
	State        string
	Kind         string
	Poster       string
	Acceptor     string
	DeadlineTick uint64
}

func NormalizeKind(k string) string {
	k = strings.TrimSpace(strings.ToUpper(k))
	switch k {
	case "GATHER", "DELIVER", "BUILD":
		return k
	default:
		return ""
	}
}

func BuildSummaries(in []SummaryInput) []map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	sort.Slice(in, func(i, j int) bool {
		return in[i].ContractID < in[j].ContractID
	})
	out := make([]map[string]interface{}, 0, len(in))
	for _, c := range in {
		out = append(out, map[string]interface{}{
			"contract_id":   c.ContractID,
			"state":         c.State,
			"kind":          c.Kind,
			"poster":        c.Poster,
			"acceptor":      c.Acceptor,
			"deadline_tick": c.DeadlineTick,
		})
	}
	return out
}

type TickDecision string

const (
	DecisionNoop              TickDecision = "NOOP"
	DecisionFailMissingTarget TickDecision = "FAIL_MISSING_TERMINAL"
	DecisionExpireOpen        TickDecision = "EXPIRE_OPEN"
	DecisionTimeoutAccepted   TickDecision = "TIMEOUT_ACCEPTED"
	DecisionCompleteDeliver   TickDecision = "COMPLETE_DELIVER"
	DecisionCompleteBuild     TickDecision = "COMPLETE_BUILD"
)

type TickInput struct {
	State          string
	Kind           string
	NowTick        uint64
	DeadlineTick   uint64
	HasTerminal    bool
	RequirementsOK bool
	BuildPlaced    bool
	BuildStable    bool
}

func DecideTick(in TickInput) TickDecision {
	if in.State != "OPEN" && in.State != "ACCEPTED" {
		return DecisionNoop
	}
	if !in.HasTerminal {
		return DecisionFailMissingTarget
	}
	if in.State == "OPEN" {
		if in.NowTick > in.DeadlineTick {
			return DecisionExpireOpen
		}
		return DecisionNoop
	}
	if in.NowTick > in.DeadlineTick {
		return DecisionTimeoutAccepted
	}
	switch in.Kind {
	case "GATHER", "DELIVER":
		if in.RequirementsOK {
			return DecisionCompleteDeliver
		}
	case "BUILD":
		if in.BuildPlaced && in.BuildStable {
			return DecisionCompleteBuild
		}
	}
	return DecisionNoop
}

type PayoutTarget string

const (
	PayoutNone     PayoutTarget = ""
	PayoutPoster   PayoutTarget = "POSTER"
	PayoutAcceptor PayoutTarget = "ACCEPTOR"
)

type SettlementPlan struct {
	MarkFailed           bool
	MarkCompleted        bool
	ConsumeRequirements  bool
	RequirementsToPoster bool
	RewardTo             PayoutTarget
	DepositTo            PayoutTarget
	PenalizeAcceptorLaw  bool
	AuditReason          string
}

func PlanSettlement(decision TickDecision) SettlementPlan {
	switch decision {
	case DecisionFailMissingTarget:
		return SettlementPlan{MarkFailed: true}
	case DecisionExpireOpen:
		return SettlementPlan{
			MarkFailed:  true,
			RewardTo:    PayoutPoster,
			AuditReason: "CONTRACT_TIMEOUT",
		}
	case DecisionTimeoutAccepted:
		return SettlementPlan{
			MarkFailed:          true,
			RewardTo:            PayoutPoster,
			DepositTo:           PayoutPoster,
			PenalizeAcceptorLaw: true,
			AuditReason:         "CONTRACT_TIMEOUT",
		}
	case DecisionCompleteDeliver:
		return SettlementPlan{
			MarkCompleted:        true,
			ConsumeRequirements:  true,
			RequirementsToPoster: true,
			RewardTo:             PayoutAcceptor,
			DepositTo:            PayoutAcceptor,
			AuditReason:          "AUTO_COMPLETE",
		}
	case DecisionCompleteBuild:
		return SettlementPlan{
			MarkCompleted: true,
			RewardTo:      PayoutAcceptor,
			DepositTo:     PayoutAcceptor,
			AuditReason:   "AUTO_COMPLETE",
		}
	default:
		return SettlementPlan{}
	}
}
