package instants

import (
	"voxelcraft.ai/internal/protocol"
	lifecyclepkg "voxelcraft.ai/internal/sim/world/feature/contracts/lifecycle"
	reppkg "voxelcraft.ai/internal/sim/world/feature/contracts/reputation"
	runtimepkg "voxelcraft.ai/internal/sim/world/feature/contracts/runtime"
	validationpkg "voxelcraft.ai/internal/sim/world/feature/contracts/validation"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
)

type ContractLifecycleEnv interface {
	GetContainerByID(id string) *modelpkg.Container
	NewContractID() string
	PutContract(c *modelpkg.Contract)
	GetContract(contractID string) *modelpkg.Contract
	RepDepositMultiplier(a *modelpkg.Agent) int
	CheckBuildContract(c *modelpkg.Contract) bool
}

type ContractLifecycleHooks struct {
	OnPosted   func(ContractPostOutcome)
	OnAccepted func(ContractAcceptOutcome)
	OnSubmitted func(ContractSubmitOutcome)
}

type ContractPostOutcome struct {
	Contract *modelpkg.Contract
	Terminal *modelpkg.Container
}

type ContractAcceptOutcome struct {
	Contract *modelpkg.Contract
	Terminal *modelpkg.Container
}

type ContractSubmitOutcome struct {
	Contract *modelpkg.Contract
	Terminal *modelpkg.Container
	Acceptor *modelpkg.Agent
}

func HandlePostContract(env ContractLifecycleEnv, ar ActionResultFn, hooks ContractLifecycleHooks, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64, dayTicks int) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "contract env unavailable"))
		return
	}
	term := env.GetContainerByID(inst.TerminalID)
	req := inventorypkg.StacksToMap(inst.Requirements)
	reward := inventorypkg.StacksToMap(inst.Reward)
	deposit := inventorypkg.StacksToMap(inst.Deposit)
	distance := 0
	terminalType := ""
	if term != nil {
		distance = modelpkg.Manhattan(a.Pos, term.Pos)
		terminalType = term.Type
	}
	prep := validationpkg.PreparePost(validationpkg.PostPrepInput{
		TerminalID:      inst.TerminalID,
		TerminalType:    terminalType,
		Distance:        distance,
		Kind:            inst.ContractKind,
		Requirements:    req,
		Reward:          reward,
		BlueprintID:     inst.BlueprintID,
		HasEnoughReward: inventorypkg.HasItems(a.Inventory, reward),
		NowTick:         nowTick,
		DeadlineTick:    inst.DeadlineTick,
		DurationTicks:   inst.DurationTicks,
		DayTicks:        dayTicks,
	})
	if ok, code, msg := validationpkg.ValidatePost(prep.Validation); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	kind := prep.ResolvedKind
	deadline := prep.Deadline

	for item, n := range reward {
		a.Inventory[item] -= n
		term.Inventory[item] += n
		term.Reserve(item, n)
	}

	cid := env.NewContractID()
	c := &modelpkg.Contract{
		ContractID:   cid,
		TerminalPos:  term.Pos,
		Poster:       a.ID,
		Kind:         kind,
		Requirements: req,
		Reward:       reward,
		Deposit:      deposit,
		CreatedTick:  nowTick,
		DeadlineTick: deadline,
		State:        modelpkg.ContractOpen,
	}
	if kind == "BUILD" {
		c.BlueprintID = inst.BlueprintID
		c.Anchor = modelpkg.Vec3i{X: inst.Anchor[0], Y: inst.Anchor[1], Z: inst.Anchor[2]}
		c.Rotation = blueprint.NormalizeRotation(inst.Rotation)
	}
	env.PutContract(c)
	if hooks.OnPosted != nil {
		hooks.OnPosted(ContractPostOutcome{Contract: c, Terminal: term})
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "contract_id": cid})
}

func HandleAcceptContract(env ContractLifecycleEnv, ar ActionResultFn, hooks ContractLifecycleHooks, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "contract env unavailable"))
		return
	}
	if ok, code, msg := validationpkg.ValidateLifecycleIDs(inst.ContractID, inst.TerminalID); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	c := env.GetContract(inst.ContractID)
	term := env.GetContainerByID(inst.TerminalID)
	state := ""
	deadline := uint64(0)
	distance := 0
	terminalType := ""
	terminalMatch := false
	if c != nil {
		state = string(c.State)
		deadline = c.DeadlineTick
		if term != nil {
			ctx := BuildTerminalContext(
				true,
				term.Type,
				Vec3{X: term.Pos.X, Y: term.Pos.Y, Z: term.Pos.Z},
				Vec3{X: c.TerminalPos.X, Y: c.TerminalPos.Y, Z: c.TerminalPos.Z},
				Vec3{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
			)
			terminalType = ctx.Type
			distance = ctx.Distance
			terminalMatch = ctx.Matches
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
		DepositMult: env.RepDepositMultiplier(a),
		Inventory:   a.Inventory,
	})
	if ok, code, msg := validationpkg.ValidateAccept(prep.Validation); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}

	for item, n := range prep.RequiredDeposit {
		a.Inventory[item] -= n
		term.Inventory[item] += n
		term.Reserve(item, n)
	}
	c.Deposit = prep.RequiredDeposit
	c.Acceptor = a.ID
	c.State = modelpkg.ContractAccepted
	if hooks.OnAccepted != nil {
		hooks.OnAccepted(ContractAcceptOutcome{Contract: c, Terminal: term})
	}
	a.AddEvent(ar(nowTick, inst.ID, true, "", "accepted"))
}

func HandleSubmitContract(env ContractLifecycleEnv, ar ActionResultFn, hooks ContractLifecycleHooks, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "contract env unavailable"))
		return
	}
	if ok, code, msg := validationpkg.ValidateLifecycleIDs(inst.ContractID, inst.TerminalID); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	c := env.GetContract(inst.ContractID)
	term := env.GetContainerByID(inst.TerminalID)
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
			ctx := BuildTerminalContext(
				true,
				terminalType,
				Vec3{X: term.Pos.X, Y: term.Pos.Y, Z: term.Pos.Z},
				Vec3{X: c.TerminalPos.X, Y: c.TerminalPos.Y, Z: c.TerminalPos.Z},
				Vec3{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
			)
			terminalMatch = ctx.Matches
			distance = ctx.Distance
			switch c.Kind {
			case "GATHER", "DELIVER":
				requirementsOK = reppkg.HasAvailable(c.Requirements, term.AvailableCount)
			case "BUILD":
				buildOK = env.CheckBuildContract(c)
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
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}

	if lifecyclepkg.NeedsRequirementsConsumption(c.Kind) {
		runtimepkg.ConsumeRequirementsToPoster(term.Inventory, c.Requirements, func(item string, n int) {
			term.AddOwed(c.Poster, item, n)
		})
	}
	runtimepkg.PayoutItems(term.Inventory, c.Reward, term.Unreserve, func(item string, n int) {
		a.Inventory[item] += n
	})
	runtimepkg.PayoutItems(term.Inventory, c.Deposit, term.Unreserve, func(item string, n int) {
		a.Inventory[item] += n
	})
	c.State = modelpkg.ContractCompleted
	if hooks.OnSubmitted != nil {
		hooks.OnSubmitted(ContractSubmitOutcome{Contract: c, Terminal: term, Acceptor: a})
	}
	a.AddEvent(ar(nowTick, inst.ID, true, "", "completed"))
}
