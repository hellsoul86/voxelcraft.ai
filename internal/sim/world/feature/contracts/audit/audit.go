package audit

import "voxelcraft.ai/internal/sim/world/feature/contracts/core"

type TickAuditInput struct {
	Decision     core.TickDecision
	AuditReason  string
	ContractID   string
	Kind         string
	Poster       string
	Acceptor     string
	State        string
	BlueprintID  string
	Anchor       [3]int
	Rotation     int
	Requirements [][]interface{}
	Reward       [][]interface{}
	Deposit      [][]interface{}
}

type TickAuditOutput struct {
	ShouldAudit      bool
	EventType        string
	Actor            string
	Reason           string
	Fields           map[string]any
	GrantTradeCredit bool
	GrantBuildCredit bool
}

func BuildPostAuditFields(contractID, terminalID, kind string, requirements, reward, deposit [][]interface{}, deadline uint64, blueprintID string, anchor [3]int, rotation int) map[string]any {
	return map[string]any{
		"contract_id":   contractID,
		"terminal_id":   terminalID,
		"kind":          kind,
		"requirements":  requirements,
		"reward":        reward,
		"deposit":       deposit,
		"deadline_tick": deadline,
		"blueprint_id":  blueprintID,
		"anchor":        anchor,
		"rotation":      rotation,
	}
}

func BuildAcceptAuditFields(contractID, terminalID, kind, poster, acceptor string, deposit [][]interface{}) map[string]any {
	return map[string]any{
		"contract_id": contractID,
		"terminal_id": terminalID,
		"kind":        kind,
		"poster":      poster,
		"acceptor":    acceptor,
		"deposit":     deposit,
	}
}

func BuildSubmitAuditFields(contractID, terminalID, kind, poster, acceptor string) map[string]any {
	return map[string]any{
		"contract_id": contractID,
		"terminal_id": terminalID,
		"kind":        kind,
		"poster":      poster,
		"acceptor":    acceptor,
	}
}

func BuildTickAudit(input TickAuditInput) TickAuditOutput {
	switch input.Decision {
	case core.DecisionExpireOpen:
		return TickAuditOutput{
			ShouldAudit: true,
			EventType:   "CONTRACT_EXPIRE",
			Actor:       "WORLD",
			Reason:      input.AuditReason,
			Fields: map[string]any{
				"contract_id": input.ContractID,
				"kind":        input.Kind,
				"poster":      input.Poster,
				"acceptor":    input.Acceptor,
				"state":       input.State,
				"reward":      input.Reward,
			},
		}
	case core.DecisionTimeoutAccepted:
		return TickAuditOutput{
			ShouldAudit: true,
			EventType:   "CONTRACT_FAIL",
			Actor:       "WORLD",
			Reason:      input.AuditReason,
			Fields: map[string]any{
				"contract_id": input.ContractID,
				"kind":        input.Kind,
				"poster":      input.Poster,
				"acceptor":    input.Acceptor,
				"state":       input.State,
				"reward":      input.Reward,
				"deposit":     input.Deposit,
			},
		}
	case core.DecisionCompleteDeliver:
		return TickAuditOutput{
			ShouldAudit:      true,
			EventType:        "CONTRACT_COMPLETE",
			Actor:            input.Acceptor,
			Reason:           input.AuditReason,
			GrantTradeCredit: true,
			Fields: map[string]any{
				"contract_id":  input.ContractID,
				"kind":         input.Kind,
				"poster":       input.Poster,
				"acceptor":     input.Acceptor,
				"state":        input.State,
				"requirements": input.Requirements,
				"reward":       input.Reward,
				"deposit":      input.Deposit,
			},
		}
	case core.DecisionCompleteBuild:
		return TickAuditOutput{
			ShouldAudit:      true,
			EventType:        "CONTRACT_COMPLETE",
			Actor:            input.Acceptor,
			Reason:           input.AuditReason,
			GrantBuildCredit: true,
			Fields: map[string]any{
				"contract_id":  input.ContractID,
				"kind":         input.Kind,
				"poster":       input.Poster,
				"acceptor":     input.Acceptor,
				"state":        input.State,
				"blueprint_id": input.BlueprintID,
				"anchor":       input.Anchor,
				"rotation":     input.Rotation,
				"reward":       input.Reward,
				"deposit":      input.Deposit,
			},
		}
	default:
		return TickAuditOutput{}
	}
}
