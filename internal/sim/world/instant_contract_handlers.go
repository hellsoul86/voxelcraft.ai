package world

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
)

func handleInstantToggleSwitch(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	target := strings.TrimSpace(inst.TargetID)
	if target == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	typ, pos, ok := parseContainerID(target)
	if !ok || typ != "SWITCH" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "invalid switch target"))
		return
	}
	if w.blockName(w.chunks.GetBlock(pos)) != "SWITCH" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "switch not found"))
		return
	}
	if Manhattan(a.Pos, pos) > 3 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
		return
	}
	if !w.canBuildAt(a.ID, pos, nowTick) {
		w.bumpRepLaw(a.ID, -1)
		if w.stats != nil {
			w.stats.RecordDenied(nowTick)
		}
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "switch toggle denied"))
		return
	}
	if w.switches == nil {
		w.switches = map[Vec3i]bool{}
	}
	on := !w.switches[pos]
	w.switches[pos] = on
	w.auditEvent(nowTick, a.ID, "SWITCH_TOGGLE", pos, "TOGGLE_SWITCH", map[string]any{
		"switch_id": switchIDAt(pos),
		"on":        on,
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "SWITCH", "switch_id": switchIDAt(pos), "pos": pos.ToArray(), "on": on})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantClaimOwed(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	// Claim owed items from a terminal container to self.
	if inst.TerminalID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing terminal_id"))
		return
	}
	c := w.getContainerByID(inst.TerminalID)
	if c == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "terminal not found"))
		return
	}
	if Manhattan(a.Pos, c.Pos) > 3 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
		return
	}
	owed := c.claimOwed(a.ID)
	if len(owed) == 0 {
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "nothing owed"))
		return
	}
	for item, n := range owed {
		if n > 0 {
			a.Inventory[item] += n
		}
	}
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "claimed"))
}

func handleInstantPostContract(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.TerminalID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing terminal_id"))
		return
	}
	term := w.getContainerByID(inst.TerminalID)
	if term == nil || term.Type != "CONTRACT_TERMINAL" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "contract terminal not found"))
		return
	}
	if Manhattan(a.Pos, term.Pos) > 3 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
		return
	}
	kind := normalizeContractKind(inst.ContractKind)
	if kind == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad contract_kind"))
		return
	}
	req := stacksToMap(inst.Requirements)
	reward := stacksToMap(inst.Reward)
	deposit := stacksToMap(inst.Deposit)
	if len(reward) == 0 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing reward"))
		return
	}
	if kind != "BUILD" && len(req) == 0 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing requirements"))
		return
	}
	var deadline uint64
	if inst.DeadlineTick != 0 {
		deadline = inst.DeadlineTick
	} else {
		dur := inst.DurationTicks
		if dur <= 0 {
			dur = w.cfg.DayTicks
		}
		deadline = nowTick + uint64(dur)
	}

	// Move reward into terminal inventory and reserve it.
	for item, n := range reward {
		if a.Inventory[item] < n {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "insufficient reward items"))
			return
		}
	}
	for item, n := range reward {
		a.Inventory[item] -= n
		term.Inventory[item] += n
		term.reserve(item, n)
	}

	cid := w.newContractID()
	c := &Contract{
		ContractID:   cid,
		TerminalPos:  term.Pos,
		Poster:       a.ID,
		Kind:         kind,
		Requirements: req,
		Reward:       reward,
		Deposit:      deposit,
		CreatedTick:  nowTick,
		DeadlineTick: deadline,
		State:        ContractOpen,
	}
	if kind == "BUILD" {
		if inst.BlueprintID == "" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing blueprint_id"))
			return
		}
		c.BlueprintID = inst.BlueprintID
		c.Anchor = Vec3i{X: inst.Anchor[0], Y: inst.Anchor[1], Z: inst.Anchor[2]}
		c.Rotation = normalizeRotation(inst.Rotation)
	}
	w.contracts[cid] = c
	w.auditEvent(nowTick, a.ID, "CONTRACT_POST", term.Pos, "POST_CONTRACT", map[string]any{
		"contract_id":   cid,
		"terminal_id":   term.ID(),
		"kind":          kind,
		"requirements":  encodeItemPairs(req),
		"reward":        encodeItemPairs(reward),
		"deposit":       encodeItemPairs(deposit),
		"deadline_tick": deadline,
		"blueprint_id":  c.BlueprintID,
		"anchor":        c.Anchor.ToArray(),
		"rotation":      c.Rotation,
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "contract_id": cid})
}

func handleInstantAcceptContract(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.ContractID == "" || inst.TerminalID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing contract_id/terminal_id"))
		return
	}
	c := w.contracts[inst.ContractID]
	if c == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "contract not found"))
		return
	}
	if c.State != ContractOpen {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "contract not open"))
		return
	}
	term := w.getContainerByID(inst.TerminalID)
	if term == nil || term.Type != "CONTRACT_TERMINAL" || term.Pos != c.TerminalPos {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "terminal mismatch"))
		return
	}
	if Manhattan(a.Pos, term.Pos) > 3 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
		return
	}
	if nowTick > c.DeadlineTick {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "contract expired"))
		return
	}
	// Take deposit from acceptor into terminal and reserve.
	// MVP: low trade rep requires higher deposit multiplier.
	reqDep := c.Deposit
	if len(c.Deposit) > 0 {
		m := w.repDepositMultiplier(a)
		if m > 1 {
			reqDep = map[string]int{}
			for item, n := range c.Deposit {
				reqDep[item] = n * m
			}
		}
	}
	for item, n := range reqDep {
		if a.Inventory[item] < n {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "insufficient deposit"))
			return
		}
	}
	for item, n := range reqDep {
		a.Inventory[item] -= n
		term.Inventory[item] += n
		term.reserve(item, n)
	}
	c.Deposit = reqDep
	c.Acceptor = a.ID
	c.State = ContractAccepted
	w.auditEvent(nowTick, a.ID, "CONTRACT_ACCEPT", term.Pos, "ACCEPT_CONTRACT", map[string]any{
		"contract_id": c.ContractID,
		"terminal_id": term.ID(),
		"kind":        c.Kind,
		"poster":      c.Poster,
		"acceptor":    c.Acceptor,
		"deposit":     encodeItemPairs(c.Deposit),
	})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "accepted"))
}

func handleInstantSubmitContract(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.ContractID == "" || inst.TerminalID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing contract_id/terminal_id"))
		return
	}
	c := w.contracts[inst.ContractID]
	if c == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "contract not found"))
		return
	}
	if c.State != ContractAccepted || c.Acceptor != a.ID {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not acceptor"))
		return
	}
	term := w.getContainerByID(inst.TerminalID)
	if term == nil || term.Type != "CONTRACT_TERMINAL" || term.Pos != c.TerminalPos {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "terminal mismatch"))
		return
	}
	if Manhattan(a.Pos, term.Pos) > 3 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
		return
	}
	if nowTick > c.DeadlineTick {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "contract expired"))
		return
	}

	ok := false
	switch c.Kind {
	case "GATHER", "DELIVER":
		ok = hasAvailable(term, c.Requirements)
	case "BUILD":
		ok = w.checkBlueprintPlaced(c.BlueprintID, c.Anchor, c.Rotation)
		if ok {
			bp, okBP := w.catalogs.Blueprints.ByID[c.BlueprintID]
			if okBP && !w.structureStable(&bp, c.Anchor, c.Rotation) {
				ok = false
			}
		}
	}
	if !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "requirements not met"))
		return
	}

	// Settle immediately (consume requirements if applicable, then pay out).
	if c.Kind == "GATHER" || c.Kind == "DELIVER" {
		for item, n := range c.Requirements {
			term.Inventory[item] -= n
			if term.Inventory[item] <= 0 {
				delete(term.Inventory, item)
			}
			term.addOwed(c.Poster, item, n)
		}
	}
	for item, n := range c.Reward {
		term.unreserve(item, n)
		term.Inventory[item] -= n
		if term.Inventory[item] <= 0 {
			delete(term.Inventory, item)
		}
		a.Inventory[item] += n
	}
	for item, n := range c.Deposit {
		term.unreserve(item, n)
		term.Inventory[item] -= n
		if term.Inventory[item] <= 0 {
			delete(term.Inventory, item)
		}
		a.Inventory[item] += n
	}
	c.State = ContractCompleted
	switch c.Kind {
	case "GATHER", "DELIVER":
		w.addTradeCredit(nowTick, a.ID, c.Poster, c.Kind)
	case "BUILD":
		w.addBuildCredit(nowTick, a.ID, c.Poster, c.Kind)
	}
	w.auditEvent(nowTick, a.ID, "CONTRACT_COMPLETE", term.Pos, "SUBMIT_CONTRACT", map[string]any{
		"contract_id": c.ContractID,
		"terminal_id": term.ID(),
		"kind":        c.Kind,
		"poster":      c.Poster,
		"acceptor":    c.Acceptor,
	})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "completed"))
}
