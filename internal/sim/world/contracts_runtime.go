package world

import (
	"fmt"

	auditpkg "voxelcraft.ai/internal/sim/world/feature/contracts/audit"
	corepkg "voxelcraft.ai/internal/sim/world/feature/contracts/core"
	reppkg "voxelcraft.ai/internal/sim/world/feature/contracts/reputation"
	runtimepkg "voxelcraft.ai/internal/sim/world/feature/contracts/runtime"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
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
	return corepkg.NormalizeKind(k)
}

func (w *World) contractSummariesForTerminal(pos Vec3i) []map[string]interface{} {
	contracts := make([]runtimepkg.SummaryContract, 0, len(w.contracts))
	for _, c := range w.contracts {
		contracts = append(contracts, runtimepkg.SummaryContract{
			ContractID:   c.ContractID,
			TerminalPos:  c.TerminalPos.ToArray(),
			State:        string(c.State),
			Kind:         c.Kind,
			Poster:       c.Poster,
			Acceptor:     c.Acceptor,
			DeadlineTick: c.DeadlineTick,
		})
	}
	return runtimepkg.BuildTerminalSummaries(pos.ToArray(), contracts)
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
				reqOK = reppkg.HasAvailable(c.Requirements, terminal.availableCount)
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
		decision := corepkg.DecideTick(corepkg.TickInput{
			State:          string(c.State),
			Kind:           c.Kind,
			NowTick:        nowTick,
			DeadlineTick:   c.DeadlineTick,
			HasTerminal:    terminal != nil,
			RequirementsOK: reqOK,
			BuildPlaced:    buildPlaced,
			BuildStable:    buildStable,
		})
		if decision == corepkg.DecisionNoop {
			continue
		}
		plan := corepkg.PlanSettlement(decision)
		if plan.MarkFailed && decision == corepkg.DecisionFailMissingTarget {
			c.State = ContractFailed
			continue
		}
		if plan.ConsumeRequirements && plan.RequirementsToPoster {
			w.contractTerminalTransfer(terminal, c.Requirements, c.Poster, false)
		}
		if plan.RewardTo != corepkg.PayoutNone {
			w.contractTerminalTransfer(terminal, c.Reward, w.contractPayoutAgent(c, plan.RewardTo), true)
		}
		if plan.DepositTo != corepkg.PayoutNone {
			w.contractTerminalTransfer(terminal, c.Deposit, w.contractPayoutAgent(c, plan.DepositTo), true)
		}
		if plan.MarkFailed {
			c.State = ContractFailed
		}
		if plan.MarkCompleted {
			c.State = ContractCompleted
		}
		if plan.PenalizeAcceptorLaw {
			w.addLawPenalty(c.Acceptor, "CONTRACT_TIMEOUT")
		}
		audit := auditpkg.BuildTickAudit(auditpkg.TickAuditInput{
			Decision:     decision,
			AuditReason:  plan.AuditReason,
			ContractID:   c.ContractID,
			Kind:         c.Kind,
			Poster:       c.Poster,
			Acceptor:     c.Acceptor,
			State:        string(c.State),
			BlueprintID:  c.BlueprintID,
			Anchor:       c.Anchor.ToArray(),
			Rotation:     c.Rotation,
			Requirements: inventorypkg.EncodeItemPairs(c.Requirements),
			Reward:       inventorypkg.EncodeItemPairs(c.Reward),
			Deposit:      inventorypkg.EncodeItemPairs(c.Deposit),
		})
		if audit.GrantTradeCredit {
			w.addTradeCredit(nowTick, c.Acceptor, c.Poster, c.Kind)
		}
		if audit.GrantBuildCredit {
			w.addBuildCredit(nowTick, c.Acceptor, c.Poster, c.Kind)
		}
		if audit.ShouldAudit {
			w.auditEvent(nowTick, audit.Actor, audit.EventType, terminal.Pos, audit.Reason, audit.Fields)
		}
	}
}

func (w *World) contractPayoutAgent(c *Contract, target corepkg.PayoutTarget) string {
	return runtimepkg.PayoutAgent(c.Poster, c.Acceptor, target)
}

func (w *World) contractTerminalTransfer(terminal *Container, items map[string]int, toAgentID string, unreserve bool) {
	if terminal == nil {
		return
	}
	runtimepkg.ApplyTerminalTransfer(
		terminal.Inventory,
		items,
		toAgentID,
		unreserve,
		terminal.unreserve,
		func(agentID, item string, n int) bool {
			a := w.agents[agentID]
			if a == nil {
				return false
			}
			a.Inventory[item] += n
			return true
		},
		func(agentID, item string, n int) {
			terminal.addOwed(agentID, item, n)
		},
	)
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
	tradeDelta, socialDelta := reppkg.TradeCreditDelta()
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
	w.bumpRepBuild(actor, reppkg.BuildCreditDelta())
	if w.stats != nil {
		w.stats.RecordTrade(nowTick)
	}
	if a := w.agents[actor]; a != nil {
		w.funOnContractComplete(a, nowTick, kind)
	}
}

func (w *World) addLawPenalty(actor, reason string) {
	tradeDelta, lawDelta := reppkg.LawPenaltyDelta(reason)
	if tradeDelta != 0 {
		w.bumpRepTrade(actor, tradeDelta)
	}
	w.bumpRepLaw(actor, lawDelta)
}

func (w *World) repDepositMultiplier(a *Agent) int {
	if a == nil {
		return 1
	}
	return reppkg.DepositMultiplier(a.RepTrade)
}

func (w *World) bumpRepTrade(agentID string, delta int) {
	a := w.agents[agentID]
	if a == nil {
		return
	}
	a.RepTrade = reppkg.ClampReputation(a.RepTrade + delta)
}

func (w *World) bumpRepBuild(agentID string, delta int) {
	a := w.agents[agentID]
	if a == nil {
		return
	}
	a.RepBuild = reppkg.ClampReputation(a.RepBuild + delta)
}

func (w *World) bumpRepSocial(agentID string, delta int) {
	a := w.agents[agentID]
	if a == nil {
		return
	}
	a.RepSocial = reppkg.ClampReputation(a.RepSocial + delta)
}

func (w *World) bumpRepLaw(agentID string, delta int) {
	a := w.agents[agentID]
	if a == nil {
		return
	}
	a.RepLaw = reppkg.ClampReputation(a.RepLaw + delta)
}
