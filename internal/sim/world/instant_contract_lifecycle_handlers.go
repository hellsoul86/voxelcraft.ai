package world

import (
	"voxelcraft.ai/internal/protocol"
	auditpkg "voxelcraft.ai/internal/sim/world/feature/contracts/audit"
	lifecyclepkg "voxelcraft.ai/internal/sim/world/feature/contracts/lifecycle"
	reppkg "voxelcraft.ai/internal/sim/world/feature/contracts/reputation"
	runtimepkg "voxelcraft.ai/internal/sim/world/feature/contracts/runtime"
	validationpkg "voxelcraft.ai/internal/sim/world/feature/contracts/validation"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
)

func handleInstantPostContract(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	term := w.getContainerByID(inst.TerminalID)
	req := inventorypkg.StacksToMap(inst.Requirements)
	reward := inventorypkg.StacksToMap(inst.Reward)
	deposit := inventorypkg.StacksToMap(inst.Deposit)
	dist := 0
	terminalType := ""
	if term != nil {
		dist = Manhattan(a.Pos, term.Pos)
		terminalType = term.Type
	}
	prep := validationpkg.PreparePost(validationpkg.PostPrepInput{
		TerminalID:      inst.TerminalID,
		TerminalType:    terminalType,
		Distance:        dist,
		Kind:            inst.ContractKind,
		Requirements:    req,
		Reward:          reward,
		BlueprintID:     inst.BlueprintID,
		HasEnoughReward: inventorypkg.HasItems(a.Inventory, reward),
		NowTick:         nowTick,
		DeadlineTick:    inst.DeadlineTick,
		DurationTicks:   inst.DurationTicks,
		DayTicks:        w.cfg.DayTicks,
	})
	if ok, code, msg := validationpkg.ValidatePost(prep.Validation); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	kind := prep.ResolvedKind
	deadline := prep.Deadline

	// Move reward into terminal inventory and reserve it.
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
		c.BlueprintID = inst.BlueprintID
		c.Anchor = Vec3i{X: inst.Anchor[0], Y: inst.Anchor[1], Z: inst.Anchor[2]}
		c.Rotation = blueprint.NormalizeRotation(inst.Rotation)
	}
	w.contracts[cid] = c
	w.auditEvent(nowTick, a.ID, "CONTRACT_POST", term.Pos, "POST_CONTRACT", auditpkg.BuildPostAuditFields(
		cid,
		term.ID(),
		kind,
		inventorypkg.EncodeItemPairs(req),
		inventorypkg.EncodeItemPairs(reward),
		inventorypkg.EncodeItemPairs(deposit),
		deadline,
		c.BlueprintID,
		c.Anchor.ToArray(),
		c.Rotation,
	))
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "contract_id": cid})
}

func handleInstantAcceptContract(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := validationpkg.ValidateLifecycleIDs(inst.ContractID, inst.TerminalID); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	c := w.contracts[inst.ContractID]
	term := w.getContainerByID(inst.TerminalID)
	state := ""
	deadline := uint64(0)
	distance := 0
	terminalType := ""
	terminalMatch := false
	if c != nil {
		state = string(c.State)
		deadline = c.DeadlineTick
		if term != nil {
			terminalType = term.Type
			distance = Manhattan(a.Pos, term.Pos)
			terminalMatch = term.Pos == c.TerminalPos
		}
	}
	prep := validationpkg.PrepareAccept(validationpkg.AcceptPrepInput{
		HasContract:     c != nil,
		State:           state,
		TerminalType:    terminalType,
		TerminalMatches: terminalMatch,
		Distance:        distance,
		NowTick:         nowTick,
		DeadlineTick:    deadline,
		BaseDeposit: func() map[string]int {
			if c == nil {
				return nil
			}
			return c.Deposit
		}(),
		DepositMult: w.repDepositMultiplier(a),
		Inventory:   a.Inventory,
	})
	if ok, code, msg := validationpkg.ValidateAccept(prep.Validation); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}

	for item, n := range prep.RequiredDeposit {
		a.Inventory[item] -= n
		term.Inventory[item] += n
		term.reserve(item, n)
	}
	c.Deposit = prep.RequiredDeposit
	c.Acceptor = a.ID
	c.State = ContractAccepted
	w.auditEvent(nowTick, a.ID, "CONTRACT_ACCEPT", term.Pos, "ACCEPT_CONTRACT",
		auditpkg.BuildAcceptAuditFields(c.ContractID, term.ID(), c.Kind, c.Poster, c.Acceptor, inventorypkg.EncodeItemPairs(c.Deposit)))
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "accepted"))
}

func handleInstantSubmitContract(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := validationpkg.ValidateLifecycleIDs(inst.ContractID, inst.TerminalID); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	c := w.contracts[inst.ContractID]
	term := w.getContainerByID(inst.TerminalID)
	state := ""
	isAcceptor := false
	terminalMatch := false
	terminalType := ""
	distance := 0
	deadline := uint64(0)
	requirementsOK := false
	buildOK := false
	kind := ""
	if c != nil {
		state = string(c.State)
		isAcceptor = c.Acceptor == a.ID
		deadline = c.DeadlineTick
		kind = c.Kind
		if term != nil {
			terminalType = term.Type
		}
		if term != nil && terminalType == "CONTRACT_TERMINAL" && term.Pos == c.TerminalPos {
			terminalMatch = true
			distance = Manhattan(a.Pos, term.Pos)
			switch c.Kind {
			case "GATHER", "DELIVER":
				requirementsOK = reppkg.HasAvailable(c.Requirements, term.availableCount)
			case "BUILD":
				buildOK = w.checkBlueprintPlaced(c.BlueprintID, c.Anchor, c.Rotation)
				if buildOK {
					bp, okBP := w.catalogs.Blueprints.ByID[c.BlueprintID]
					if okBP && !w.structureStable(&bp, c.Anchor, c.Rotation) {
						buildOK = false
					}
				}
			}
		}
	}
	validation := validationpkg.PrepareSubmitValidation(validationpkg.SubmitPrepInput{
		HasContract:     c != nil,
		State:           state,
		IsAcceptor:      isAcceptor,
		TerminalType:    terminalType,
		TerminalMatches: terminalMatch,
		Distance:        distance,
		NowTick:         nowTick,
		DeadlineTick:    deadline,
		Kind:            kind,
		RequirementsOK:  requirementsOK,
		BuildOK:         buildOK,
	})
	if ok, code, msg := validationpkg.ValidateSubmit(validation); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}

	// Settle immediately (consume requirements if applicable, then pay out).
	if lifecyclepkg.NeedsRequirementsConsumption(c.Kind) {
		runtimepkg.ConsumeRequirementsToPoster(term.Inventory, c.Requirements, func(item string, n int) {
			term.addOwed(c.Poster, item, n)
		})
	}
	runtimepkg.PayoutItems(term.Inventory, c.Reward, term.unreserve, func(item string, n int) {
		a.Inventory[item] += n
	})
	runtimepkg.PayoutItems(term.Inventory, c.Deposit, term.unreserve, func(item string, n int) {
		a.Inventory[item] += n
	})
	c.State = ContractCompleted
	switch c.Kind {
	case "GATHER", "DELIVER":
		w.addTradeCredit(nowTick, a.ID, c.Poster, c.Kind)
	case "BUILD":
		w.addBuildCredit(nowTick, a.ID, c.Poster, c.Kind)
	}
	w.auditEvent(nowTick, a.ID, "CONTRACT_COMPLETE", term.Pos, "SUBMIT_CONTRACT",
		auditpkg.BuildSubmitAuditFields(c.ContractID, term.ID(), c.Kind, c.Poster, c.Acceptor))
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "completed"))
}
