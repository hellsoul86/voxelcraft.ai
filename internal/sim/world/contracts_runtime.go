package world

import (
	"fmt"

	"voxelcraft.ai/internal/sim/world/feature/contracts"
	"voxelcraft.ai/internal/sim/world/feature/economy"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
)

type ContractState string

const (
	ContractOpen      ContractState = "OPEN"
	ContractAccepted  ContractState = "ACCEPTED"
	ContractCompleted ContractState = "COMPLETED"
	ContractFailed    ContractState = "FAILED"
)

type Contract struct {
	ContractID  string
	TerminalPos Vec3i
	Poster      string
	Acceptor    string

	Kind         string
	Requirements map[string]int
	Reward       map[string]int
	Deposit      map[string]int

	// BUILD contracts:
	BlueprintID string
	Anchor      Vec3i
	Rotation    int

	CreatedTick  uint64
	DeadlineTick uint64
	State        ContractState
}

func (w *World) newContractID() string {
	n := w.nextContractNum.Add(1)
	return fmt.Sprintf("C%06d", n)
}

func normalizeContractKind(k string) string {
	return contracts.NormalizeKind(k)
}

func (w *World) contractSummariesForTerminal(pos Vec3i) []map[string]interface{} {
	entries := make([]contracts.SummaryInput, 0, 16)
	for _, c := range w.contracts {
		if c.TerminalPos != pos {
			continue
		}
		entries = append(entries, contracts.SummaryInput{
			ContractID:   c.ContractID,
			State:        string(c.State),
			Kind:         c.Kind,
			Poster:       c.Poster,
			Acceptor:     c.Acceptor,
			DeadlineTick: c.DeadlineTick,
		})
	}
	return contracts.BuildSummaries(entries)
}

func (w *World) tickContracts(nowTick uint64) {
	for _, c := range w.contracts {
		if c.State != ContractOpen && c.State != ContractAccepted {
			continue
		}
		terminal := w.containers[c.TerminalPos]
		reqOK := false
		buildPlaced := false
		buildStable := false
		if terminal != nil && c.State == ContractAccepted {
			switch c.Kind {
			case "GATHER", "DELIVER":
				reqOK = contracts.HasAvailable(c.Requirements, terminal.availableCount)
			case "BUILD":
				buildPlaced = w.checkBlueprintPlaced(c.BlueprintID, c.Anchor, c.Rotation)
				if buildPlaced {
					bp, okBP := w.catalogs.Blueprints.ByID[c.BlueprintID]
					if okBP && w.structureStable(&bp, c.Anchor, c.Rotation) {
						buildStable = true
					}
				}
			}
		}
		decision := contracts.DecideTick(contracts.TickInput{
			State:          string(c.State),
			Kind:           c.Kind,
			NowTick:        nowTick,
			DeadlineTick:   c.DeadlineTick,
			HasTerminal:    terminal != nil,
			RequirementsOK: reqOK,
			BuildPlaced:    buildPlaced,
			BuildStable:    buildStable,
		})
		switch decision {
		case contracts.DecisionFailMissingTarget:
			c.State = ContractFailed
			continue
		case contracts.DecisionExpireOpen:
			// Refund reserved reward to poster.
			for item, n := range c.Reward {
				terminal.unreserve(item, n)
				terminal.Inventory[item] -= n
				if terminal.Inventory[item] <= 0 {
					delete(terminal.Inventory, item)
				}
				if poster := w.agents[c.Poster]; poster != nil {
					poster.Inventory[item] += n
				} else {
					terminal.addOwed(c.Poster, item, n)
				}
			}
			c.State = ContractFailed
			w.auditEvent(nowTick, "WORLD", "CONTRACT_EXPIRE", terminal.Pos, "CONTRACT_TIMEOUT", map[string]any{
				"contract_id": c.ContractID,
				"kind":        c.Kind,
				"poster":      c.Poster,
				"acceptor":    c.Acceptor,
				"state":       string(c.State),
				"reward":      economy.EncodeItemPairs(c.Reward),
			})
		case contracts.DecisionTimeoutAccepted:
			// Timeout: reward back to poster, deposit to poster.
			for item, n := range c.Reward {
				terminal.unreserve(item, n)
				terminal.Inventory[item] -= n
				if terminal.Inventory[item] <= 0 {
					delete(terminal.Inventory, item)
				}
				if poster := w.agents[c.Poster]; poster != nil {
					poster.Inventory[item] += n
				} else {
					terminal.addOwed(c.Poster, item, n)
				}
			}
			for item, n := range c.Deposit {
				terminal.unreserve(item, n)
				terminal.Inventory[item] -= n
				if terminal.Inventory[item] <= 0 {
					delete(terminal.Inventory, item)
				}
				if poster := w.agents[c.Poster]; poster != nil {
					poster.Inventory[item] += n
				} else {
					terminal.addOwed(c.Poster, item, n)
				}
			}
			c.State = ContractFailed
			w.addLawPenalty(c.Acceptor, "CONTRACT_TIMEOUT")
			w.auditEvent(nowTick, "WORLD", "CONTRACT_FAIL", terminal.Pos, "CONTRACT_TIMEOUT", map[string]any{
				"contract_id": c.ContractID,
				"kind":        c.Kind,
				"poster":      c.Poster,
				"acceptor":    c.Acceptor,
				"state":       string(c.State),
				"reward":      economy.EncodeItemPairs(c.Reward),
				"deposit":     economy.EncodeItemPairs(c.Deposit),
			})
		case contracts.DecisionCompleteDeliver:
			// Consume requirements from terminal.
			for item, n := range c.Requirements {
				terminal.Inventory[item] -= n
				if terminal.Inventory[item] <= 0 {
					delete(terminal.Inventory, item)
				}
				if poster := w.agents[c.Poster]; poster != nil {
					poster.Inventory[item] += n
				} else {
					terminal.addOwed(c.Poster, item, n)
				}
			}
			// Release reward + deposit to acceptor.
			for item, n := range c.Reward {
				terminal.unreserve(item, n)
				terminal.Inventory[item] -= n
				if terminal.Inventory[item] <= 0 {
					delete(terminal.Inventory, item)
				}
				if acc := w.agents[c.Acceptor]; acc != nil {
					acc.Inventory[item] += n
				} else {
					terminal.addOwed(c.Acceptor, item, n)
				}
			}
			for item, n := range c.Deposit {
				terminal.unreserve(item, n)
				terminal.Inventory[item] -= n
				if terminal.Inventory[item] <= 0 {
					delete(terminal.Inventory, item)
				}
				if acc := w.agents[c.Acceptor]; acc != nil {
					acc.Inventory[item] += n
				} else {
					terminal.addOwed(c.Acceptor, item, n)
				}
			}
			c.State = ContractCompleted
			w.addTradeCredit(nowTick, c.Acceptor, c.Poster, c.Kind)
			w.auditEvent(nowTick, c.Acceptor, "CONTRACT_COMPLETE", terminal.Pos, "AUTO_COMPLETE", map[string]any{
				"contract_id":  c.ContractID,
				"kind":         c.Kind,
				"poster":       c.Poster,
				"acceptor":     c.Acceptor,
				"state":        string(c.State),
				"requirements": economy.EncodeItemPairs(c.Requirements),
				"reward":       economy.EncodeItemPairs(c.Reward),
				"deposit":      economy.EncodeItemPairs(c.Deposit),
			})
		case contracts.DecisionCompleteBuild:
			for item, n := range c.Reward {
				terminal.unreserve(item, n)
				terminal.Inventory[item] -= n
				if terminal.Inventory[item] <= 0 {
					delete(terminal.Inventory, item)
				}
				if acc := w.agents[c.Acceptor]; acc != nil {
					acc.Inventory[item] += n
				} else {
					terminal.addOwed(c.Acceptor, item, n)
				}
			}
			for item, n := range c.Deposit {
				terminal.unreserve(item, n)
				terminal.Inventory[item] -= n
				if terminal.Inventory[item] <= 0 {
					delete(terminal.Inventory, item)
				}
				if acc := w.agents[c.Acceptor]; acc != nil {
					acc.Inventory[item] += n
				} else {
					terminal.addOwed(c.Acceptor, item, n)
				}
			}
			c.State = ContractCompleted
			w.addBuildCredit(nowTick, c.Acceptor, c.Poster, c.Kind)
			w.auditEvent(nowTick, c.Acceptor, "CONTRACT_COMPLETE", terminal.Pos, "AUTO_COMPLETE", map[string]any{
				"contract_id":  c.ContractID,
				"kind":         c.Kind,
				"poster":       c.Poster,
				"acceptor":     c.Acceptor,
				"state":        string(c.State),
				"blueprint_id": c.BlueprintID,
				"anchor":       c.Anchor.ToArray(),
				"rotation":     c.Rotation,
				"reward":       economy.EncodeItemPairs(c.Reward),
				"deposit":      economy.EncodeItemPairs(c.Deposit),
			})
		case contracts.DecisionNoop:
			continue
		}
	}
}

func (w *World) checkBlueprintPlaced(id string, anchor Vec3i, rotation int) bool {
	bp, ok := w.catalogs.Blueprints.ByID[id]
	if !ok {
		return false
	}
	blocks := make([]blueprint.PlacementBlock, 0, len(bp.Blocks))
	for _, b := range bp.Blocks {
		blocks = append(blocks, blueprint.PlacementBlock{
			Pos:   b.Pos,
			Block: b.Block,
		})
	}
	return blueprint.CheckPlaced(
		func(x, y, z int) uint16 { return w.chunks.GetBlock(Vec3i{X: x, Y: y, Z: z}) },
		w.catalogs.Blocks.Index,
		blocks,
		anchor.ToArray(),
		rotation,
	)
}

// MVP placeholders for reputation systems; keep hooks for future.
func (w *World) addTradeCredit(nowTick uint64, actor, counterparty string, kind string) {
	_ = counterparty
	tradeDelta, socialDelta := contracts.TradeCreditDelta()
	w.bumpRepTrade(actor, tradeDelta)
	w.bumpRepSocial(actor, socialDelta)
	if w.stats != nil {
		w.stats.RecordTrade(nowTick)
	}
	if a := w.agents[actor]; a != nil {
		w.funOnContractComplete(a, nowTick, kind)
	}
}

func (w *World) addBuildCredit(nowTick uint64, actor, counterparty string, kind string) {
	_ = counterparty
	w.bumpRepBuild(actor, contracts.BuildCreditDelta())
	if w.stats != nil {
		w.stats.RecordTrade(nowTick)
	}
	if a := w.agents[actor]; a != nil {
		w.funOnContractComplete(a, nowTick, kind)
	}
}

func (w *World) addLawPenalty(actor, reason string) {
	tradeDelta, lawDelta := contracts.LawPenaltyDelta(reason)
	if tradeDelta != 0 {
		w.bumpRepTrade(actor, tradeDelta)
	}
	w.bumpRepLaw(actor, lawDelta)
}

func (w *World) repDepositMultiplier(a *Agent) int {
	if a == nil {
		return 1
	}
	return contracts.DepositMultiplier(a.RepTrade)
}

func (w *World) bumpRepTrade(agentID string, delta int) {
	a := w.agents[agentID]
	if a == nil {
		return
	}
	a.RepTrade = contracts.ClampReputation(a.RepTrade + delta)
}

func (w *World) bumpRepBuild(agentID string, delta int) {
	a := w.agents[agentID]
	if a == nil {
		return
	}
	a.RepBuild = contracts.ClampReputation(a.RepBuild + delta)
}

func (w *World) bumpRepSocial(agentID string, delta int) {
	a := w.agents[agentID]
	if a == nil {
		return
	}
	a.RepSocial = contracts.ClampReputation(a.RepSocial + delta)
}

func (w *World) bumpRepLaw(agentID string, delta int) {
	a := w.agents[agentID]
	if a == nil {
		return
	}
	a.RepLaw = contracts.ClampReputation(a.RepLaw + delta)
}
