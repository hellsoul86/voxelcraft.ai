package world

import (
	"fmt"
	"strings"
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
	k = strings.TrimSpace(strings.ToUpper(k))
	switch k {
	case "GATHER", "DELIVER", "BUILD":
		return k
	default:
		return ""
	}
}

func (w *World) contractSummariesForTerminal(pos Vec3i) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, 16)
	for _, c := range w.contracts {
		if c.TerminalPos != pos {
			continue
		}
		out = append(out, map[string]interface{}{
			"contract_id":   c.ContractID,
			"state":         string(c.State),
			"kind":          c.Kind,
			"poster":        c.Poster,
			"acceptor":      c.Acceptor,
			"deadline_tick": c.DeadlineTick,
		})
	}
	return out
}

func (w *World) tickContracts(nowTick uint64) {
	for _, c := range w.contracts {
		if c.State != ContractOpen && c.State != ContractAccepted {
			continue
		}
		terminal := w.containers[c.TerminalPos]
		if terminal == nil {
			// Terminal lost; fail safely.
			c.State = ContractFailed
			continue
		}
		if c.State == ContractOpen {
			// Nothing to do until accepted or deadline.
			if nowTick > c.DeadlineTick {
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
					"reward":      encodeItemPairs(c.Reward),
				})
			}
			continue
		}

		// ACCEPTED
		if nowTick > c.DeadlineTick {
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
				"reward":      encodeItemPairs(c.Reward),
				"deposit":     encodeItemPairs(c.Deposit),
			})
			continue
		}

		switch c.Kind {
		case "GATHER", "DELIVER":
			if hasAvailable(terminal, c.Requirements) {
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
					"requirements": encodeItemPairs(c.Requirements),
					"reward":       encodeItemPairs(c.Reward),
					"deposit":      encodeItemPairs(c.Deposit),
				})
			}
		case "BUILD":
			// Validate structure roughly by checking all blueprint blocks at anchor.
			ok := w.checkBlueprintPlaced(c.BlueprintID, c.Anchor, c.Rotation)
			if ok {
				bp, okBP := w.catalogs.Blueprints.ByID[c.BlueprintID]
				if okBP && !w.structureStable(&bp, c.Anchor, c.Rotation) {
					ok = false
				}
			}
			if ok {
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
					"reward":       encodeItemPairs(c.Reward),
					"deposit":      encodeItemPairs(c.Deposit),
				})
			}
		}
	}
}

func (w *World) checkBlueprintPlaced(id string, anchor Vec3i, rotation int) bool {
	rot := normalizeRotation(rotation)
	bp, ok := w.catalogs.Blueprints.ByID[id]
	if !ok {
		return false
	}
	for _, b := range bp.Blocks {
		want, ok := w.catalogs.Blocks.Index[b.Block]
		if !ok {
			return false
		}
		off := rotateOffset(b.Pos, rot)
		pos := Vec3i{X: anchor.X + off[0], Y: anchor.Y + off[1], Z: anchor.Z + off[2]}
		if w.chunks.GetBlock(pos) != want {
			return false
		}
	}
	return true
}

// MVP placeholders for reputation systems; keep hooks for future.
func (w *World) addTradeCredit(nowTick uint64, actor, counterparty string, kind string) {
	_ = counterparty
	w.bumpRepTrade(actor, 6)
	w.bumpRepSocial(actor, 2)
	if w.stats != nil {
		w.stats.RecordTrade(nowTick)
	}
	if a := w.agents[actor]; a != nil {
		w.funOnContractComplete(a, nowTick, kind)
	}
}

func (w *World) addBuildCredit(nowTick uint64, actor, counterparty string, kind string) {
	_ = counterparty
	w.bumpRepBuild(actor, 6)
	if w.stats != nil {
		w.stats.RecordTrade(nowTick)
	}
	if a := w.agents[actor]; a != nil {
		w.funOnContractComplete(a, nowTick, kind)
	}
}

func (w *World) addLawPenalty(actor, reason string) {
	// Penalties should be meaningful but not instantly ruin an agent.
	switch reason {
	case "CONTRACT_TIMEOUT":
		w.bumpRepTrade(actor, -12)
		w.bumpRepLaw(actor, -8)
	default:
		w.bumpRepLaw(actor, -4)
	}
}

func hasAvailable(c *Container, req map[string]int) bool {
	if len(req) == 0 {
		return true
	}
	for item, n := range req {
		if c.availableCount(item) < n {
			return false
		}
	}
	return true
}
