package runtime

import corepkg "voxelcraft.ai/internal/sim/world/feature/contracts/core"

type SummaryContract struct {
	ContractID   string
	TerminalPos  [3]int
	State        string
	Kind         string
	Poster       string
	Acceptor     string
	DeadlineTick uint64
}

func BuildTerminalSummaries(terminalPos [3]int, contracts []SummaryContract) []map[string]interface{} {
	entries := make([]corepkg.SummaryInput, 0, len(contracts))
	for _, c := range contracts {
		if c.TerminalPos != terminalPos {
			continue
		}
		entries = append(entries, corepkg.SummaryInput{
			ContractID:   c.ContractID,
			State:        c.State,
			Kind:         c.Kind,
			Poster:       c.Poster,
			Acceptor:     c.Acceptor,
			DeadlineTick: c.DeadlineTick,
		})
	}
	return corepkg.BuildSummaries(entries)
}

func PayoutAgent(poster string, acceptor string, target corepkg.PayoutTarget) string {
	switch target {
	case corepkg.PayoutPoster:
		return poster
	case corepkg.PayoutAcceptor:
		return acceptor
	default:
		return ""
	}
}

func ApplyTerminalTransfer(
	terminalInventory map[string]int,
	items map[string]int,
	toAgentID string,
	unreserve bool,
	unreserveItem func(item string, n int),
	addAgentItem func(agentID, item string, n int) bool,
	addOwedItem func(agentID, item string, n int),
) {
	if len(items) == 0 || toAgentID == "" || terminalInventory == nil {
		return
	}
	for item, n := range items {
		if n <= 0 {
			continue
		}
		if unreserve && unreserveItem != nil {
			unreserveItem(item, n)
		}
		terminalInventory[item] -= n
		if terminalInventory[item] <= 0 {
			delete(terminalInventory, item)
		}
		added := false
		if addAgentItem != nil {
			added = addAgentItem(toAgentID, item, n)
		}
		if !added && addOwedItem != nil {
			addOwedItem(toAgentID, item, n)
		}
	}
}

func ConsumeRequirementsToPoster(
	terminalInventory map[string]int,
	requirements map[string]int,
	addPosterItem func(item string, n int),
) {
	if terminalInventory == nil || len(requirements) == 0 {
		return
	}
	for item, n := range requirements {
		if n <= 0 {
			continue
		}
		terminalInventory[item] -= n
		if terminalInventory[item] <= 0 {
			delete(terminalInventory, item)
		}
		if addPosterItem != nil {
			addPosterItem(item, n)
		}
	}
}

func PayoutItems(
	terminalInventory map[string]int,
	payout map[string]int,
	unreserveItem func(item string, n int),
	addReceiverItem func(item string, n int),
) {
	if terminalInventory == nil || len(payout) == 0 {
		return
	}
	for item, n := range payout {
		if n <= 0 {
			continue
		}
		if unreserveItem != nil {
			unreserveItem(item, n)
		}
		terminalInventory[item] -= n
		if terminalInventory[item] <= 0 {
			delete(terminalInventory, item)
		}
		if addReceiverItem != nil {
			addReceiverItem(item, n)
		}
	}
}
