package world

import (
	"errors"

	"voxelcraft.ai/internal/protocol"
	auditpkg "voxelcraft.ai/internal/sim/world/feature/contracts/audit"
	contractinstantspkg "voxelcraft.ai/internal/sim/world/feature/contracts/instants"
	lifecyclepkg "voxelcraft.ai/internal/sim/world/feature/contracts/lifecycle"
	reppkg "voxelcraft.ai/internal/sim/world/feature/contracts/reputation"
	runtimepkg "voxelcraft.ai/internal/sim/world/feature/contracts/runtime"
	validationpkg "voxelcraft.ai/internal/sim/world/feature/contracts/validation"
	conveyorinstantspkg "voxelcraft.ai/internal/sim/world/feature/conveyor/runtime"
	economyinstantspkg "voxelcraft.ai/internal/sim/world/feature/economy/instants"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	taxpkg "voxelcraft.ai/internal/sim/world/feature/economy/tax"
	governanceinstantspkg "voxelcraft.ai/internal/sim/world/feature/governance/instants"
	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
	postingpkg "voxelcraft.ai/internal/sim/world/feature/observer/posting"
	sessioninstantspkg "voxelcraft.ai/internal/sim/world/feature/session/instants"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
)

type instantHandler func(*World, *Agent, protocol.InstantReq, uint64)

var instantDispatch = map[string]instantHandler{
	InstantTypeSay:            handleInstantSay,
	InstantTypeWhisper:        handleInstantWhisper,
	InstantTypeEat:            handleInstantEat,
	InstantTypeSaveMemory:     handleInstantSaveMemory,
	InstantTypeLoadMemory:     handleInstantLoadMemory,
	InstantTypeOfferTrade:     handleInstantOfferTrade,
	InstantTypeAcceptTrade:    handleInstantAcceptTrade,
	InstantTypeDeclineTrade:   handleInstantDeclineTrade,
	InstantTypePostBoard:      handleInstantPostBoard,
	InstantTypeSearchBoard:    handleInstantSearchBoard,
	InstantTypeSetSign:        handleInstantSetSign,
	InstantTypeToggleSwitch:   handleInstantToggleSwitch,
	InstantTypeClaimOwed:      handleInstantClaimOwed,
	InstantTypePostContract:   handleInstantPostContract,
	InstantTypeAcceptContract: handleInstantAcceptContract,
	InstantTypeSubmitContract: handleInstantSubmitContract,
	InstantTypeSetPermissions: handleInstantSetPermissions,
	InstantTypeUpgradeClaim:   handleInstantUpgradeClaim,
	InstantTypeAddMember:      handleInstantAddMember,
	InstantTypeRemoveMember:   handleInstantRemoveMember,
	InstantTypeCreateOrg:      handleInstantCreateOrg,
	InstantTypeJoinOrg:        handleInstantJoinOrg,
	InstantTypeOrgDeposit:     handleInstantOrgDeposit,
	InstantTypeOrgWithdraw:    handleInstantOrgWithdraw,
	InstantTypeLeaveOrg:       handleInstantLeaveOrg,
	InstantTypeDeedLand:       handleInstantDeedLand,
	InstantTypeProposeLaw:     handleInstantProposeLaw,
	InstantTypeVote:           handleInstantVote,
}

type sessionInstantsWorldEnv struct {
	w *World
}

func (e sessionInstantsWorldEnv) IsOrgMember(agentID, orgID string) bool {
	if e.w == nil {
		return false
	}
	return e.w.isOrgMember(agentID, orgID)
}

func (e sessionInstantsWorldEnv) PermissionsFor(agentID string, pos Vec3i) map[string]bool {
	if e.w == nil {
		return map[string]bool{}
	}
	_, perms := e.w.permissionsFor(agentID, pos)
	return perms
}

func (e sessionInstantsWorldEnv) BroadcastChat(nowTick uint64, from *Agent, channel, text string) {
	if e.w == nil {
		return
	}
	e.w.broadcastChat(nowTick, from, channel, text)
}

func (e sessionInstantsWorldEnv) AgentByID(agentID string) *Agent {
	if e.w == nil {
		return nil
	}
	return e.w.agents[agentID]
}

type economyInstantsWorldEnv struct {
	w *World
}

func (e economyInstantsWorldEnv) PermissionsFor(agentID string, pos Vec3i) map[string]bool {
	if e.w == nil {
		return map[string]bool{}
	}
	_, perms := e.w.permissionsFor(agentID, pos)
	return perms
}

func (e economyInstantsWorldEnv) AgentByID(agentID string) *Agent {
	if e.w == nil {
		return nil
	}
	return e.w.agents[agentID]
}

func (e economyInstantsWorldEnv) NewTradeID() string {
	if e.w == nil {
		return ""
	}
	return economyinstantspkg.SimpleTradeIDFromCounter(e.w.nextTradeNum.Add(1))
}

func (e economyInstantsWorldEnv) PutTrade(tr *Trade) {
	if e.w == nil || tr == nil {
		return
	}
	e.w.trades[tr.TradeID] = tr
}

func (e economyInstantsWorldEnv) GetTrade(tradeID string) *Trade {
	if e.w == nil {
		return nil
	}
	return e.w.trades[tradeID]
}

func (e economyInstantsWorldEnv) DeleteTrade(tradeID string) {
	if e.w == nil {
		return
	}
	delete(e.w.trades, tradeID)
}

type observerPostingWorldEnv struct {
	w *World
}

func (e observerPostingWorldEnv) ParseContainerID(id string) (typ string, pos Vec3i, ok bool) {
	if e.w == nil {
		return "", Vec3i{}, false
	}
	return parseContainerID(id)
}

func (e observerPostingWorldEnv) CanonicalBoardID(pos Vec3i) string {
	return boardIDAt(pos)
}

func (e observerPostingWorldEnv) BlockNameAt(pos Vec3i) string {
	if e.w == nil {
		return ""
	}
	return e.w.blockName(e.w.chunks.GetBlock(pos))
}

func (e observerPostingWorldEnv) Distance(a Vec3i, b Vec3i) int {
	return Manhattan(a, b)
}

func (e observerPostingWorldEnv) PostingAllowed(agentID string, pos Vec3i) bool {
	if e.w == nil {
		return false
	}
	land := e.w.landAt(pos)
	return land == nil || e.w.isLandMember(agentID, land) || land.Flags.AllowTrade
}

func (e observerPostingWorldEnv) GetBoard(boardID string) *Board {
	if e.w == nil {
		return nil
	}
	return e.w.boards[boardID]
}

func (e observerPostingWorldEnv) EnsureBoard(pos Vec3i) *Board {
	if e.w == nil {
		return nil
	}
	return e.w.ensureBoard(pos)
}

func (e observerPostingWorldEnv) PutBoard(boardID string, board *Board) {
	if e.w == nil || board == nil {
		return
	}
	e.w.boards[boardID] = board
}

func (e observerPostingWorldEnv) NewPostID() string {
	if e.w == nil {
		return ""
	}
	return e.w.newPostID()
}

func (e observerPostingWorldEnv) AuditBoardPost(nowTick uint64, actorID string, pos Vec3i, boardID string, postID string, title string) {
	if e.w == nil {
		return
	}
	e.w.auditEvent(nowTick, actorID, "BOARD_POST", pos, "POST_BOARD", map[string]any{
		"board_id": boardID,
		"post_id":  postID,
		"title":    title,
	})
}

func (e observerPostingWorldEnv) CanBuildAt(agentID string, pos Vec3i, nowTick uint64) bool {
	if e.w == nil {
		return false
	}
	return e.w.canBuildAt(agentID, pos, nowTick)
}

func (e observerPostingWorldEnv) EnsureSign(pos Vec3i) *Sign {
	if e.w == nil {
		return nil
	}
	return e.w.ensureSign(pos)
}

func (e observerPostingWorldEnv) SignIDAt(pos Vec3i) string {
	return signIDAt(pos)
}

func (e observerPostingWorldEnv) AuditSignSet(nowTick uint64, actorID string, pos Vec3i, signID string, text string) {
	if e.w == nil {
		return
	}
	e.w.auditEvent(nowTick, actorID, "SIGN_SET", pos, "SET_SIGN", map[string]any{
		"sign_id":     signID,
		"text":        text,
		"text_length": len(text),
	})
}

func (e observerPostingWorldEnv) BumpLawRep(agentID string, delta int) {
	if e.w == nil {
		return
	}
	e.w.bumpRepLaw(agentID, delta)
}

func (e observerPostingWorldEnv) RecordDenied(nowTick uint64) {
	if e.w == nil || e.w.stats == nil {
		return
	}
	e.w.stats.RecordDenied(nowTick)
}

type conveyorInstantsWorldEnv struct {
	w *World
}

func (e conveyorInstantsWorldEnv) ParseContainerID(id string) (typ string, pos Vec3i, ok bool) {
	if e.w == nil {
		return "", Vec3i{}, false
	}
	return parseContainerID(id)
}

func (e conveyorInstantsWorldEnv) BlockNameAt(pos Vec3i) string {
	if e.w == nil {
		return ""
	}
	return e.w.blockName(e.w.chunks.GetBlock(pos))
}

func (e conveyorInstantsWorldEnv) Distance(a Vec3i, b Vec3i) int {
	return Manhattan(a, b)
}

func (e conveyorInstantsWorldEnv) CanBuildAt(agentID string, pos Vec3i, nowTick uint64) bool {
	if e.w == nil {
		return false
	}
	return e.w.canBuildAt(agentID, pos, nowTick)
}

func (e conveyorInstantsWorldEnv) SwitchStateAt(pos Vec3i) bool {
	if e.w == nil {
		return false
	}
	if e.w.switches == nil {
		return false
	}
	return e.w.switches[pos]
}

func (e conveyorInstantsWorldEnv) SetSwitchState(pos Vec3i, on bool) {
	if e.w == nil {
		return
	}
	if e.w.switches == nil {
		e.w.switches = map[Vec3i]bool{}
	}
	e.w.switches[pos] = on
}

func (e conveyorInstantsWorldEnv) SwitchIDAt(pos Vec3i) string {
	return switchIDAt(pos)
}

func (e conveyorInstantsWorldEnv) AuditSwitchToggle(nowTick uint64, actorID string, pos Vec3i, switchID string, on bool) {
	if e.w == nil {
		return
	}
	e.w.auditEvent(nowTick, actorID, "SWITCH_TOGGLE", pos, "TOGGLE_SWITCH", map[string]any{
		"switch_id": switchID,
		"on":        on,
	})
}

func (e conveyorInstantsWorldEnv) BumpLawRep(agentID string, delta int) {
	if e.w == nil {
		return
	}
	e.w.bumpRepLaw(agentID, delta)
}

func (e conveyorInstantsWorldEnv) RecordDenied(nowTick uint64) {
	if e.w == nil || e.w.stats == nil {
		return
	}
	e.w.stats.RecordDenied(nowTick)
}

type contractInstantsWorldEnv struct {
	w *World
}

func (e contractInstantsWorldEnv) GetContainerByID(id string) *Container {
	if e.w == nil {
		return nil
	}
	return e.w.getContainerByID(id)
}

func (e contractInstantsWorldEnv) Distance(a Vec3i, b Vec3i) int {
	return Manhattan(a, b)
}

type governanceOrgInstantsWorldEnv struct {
	w *World
}

func (e governanceOrgInstantsWorldEnv) NewOrgID() string {
	if e.w == nil {
		return ""
	}
	return e.w.newOrgID()
}

func (e governanceOrgInstantsWorldEnv) GetOrg(orgID string) *Organization {
	if e.w == nil {
		return nil
	}
	return e.w.orgByID(orgID)
}

func (e governanceOrgInstantsWorldEnv) PutOrg(org *Organization) {
	if e.w == nil || org == nil {
		return
	}
	e.w.orgs[org.OrgID] = org
}

func (e governanceOrgInstantsWorldEnv) DeleteOrg(orgID string) {
	if e.w == nil {
		return
	}
	delete(e.w.orgs, orgID)
}

func (e governanceOrgInstantsWorldEnv) OrgTreasury(org *Organization) map[string]int {
	if e.w == nil {
		return nil
	}
	return e.w.orgTreasury(org)
}

func (e governanceOrgInstantsWorldEnv) IsOrgMember(agentID string, orgID string) bool {
	if e.w == nil {
		return false
	}
	return e.w.isOrgMember(agentID, orgID)
}

func (e governanceOrgInstantsWorldEnv) IsOrgAdmin(agentID string, orgID string) bool {
	if e.w == nil {
		return false
	}
	return e.w.isOrgAdmin(agentID, orgID)
}

func (e governanceOrgInstantsWorldEnv) AuditOrgEvent(nowTick uint64, actorID string, action string, reason string, details map[string]any) {
	if e.w == nil {
		return
	}
	pos := Vec3i{}
	if a := e.w.agents[actorID]; a != nil {
		pos = a.Pos
	}
	e.w.auditEvent(nowTick, actorID, action, pos, reason, details)
}

type governanceClaimInstantsWorldEnv struct {
	w *World
}

func (e governanceClaimInstantsWorldEnv) GetLand(landID string) *LandClaim {
	if e.w == nil {
		return nil
	}
	return e.w.claims[landID]
}

func (e governanceClaimInstantsWorldEnv) IsLandAdmin(agentID string, land *LandClaim) bool {
	if e.w == nil {
		return false
	}
	return e.w.isLandAdmin(agentID, land)
}

func (e governanceClaimInstantsWorldEnv) BlockNameAt(pos Vec3i) string {
	if e.w == nil {
		return ""
	}
	return e.w.blockName(e.w.chunks.GetBlock(pos))
}

func (e governanceClaimInstantsWorldEnv) ClaimRecords() []governanceinstantspkg.ClaimRecord {
	if e.w == nil {
		return nil
	}
	records := make([]governanceinstantspkg.ClaimRecord, 0, len(e.w.claims))
	for _, c := range e.w.claims {
		if c == nil {
			continue
		}
		records = append(records, governanceinstantspkg.ClaimRecord{
			LandID:  c.LandID,
			AnchorX: c.Anchor.X,
			AnchorZ: c.Anchor.Z,
			Radius:  c.Radius,
		})
	}
	return records
}

func (e governanceClaimInstantsWorldEnv) OwnerExists(ownerID string) bool {
	if e.w == nil {
		return false
	}
	return e.w.agents[ownerID] != nil || e.w.orgByID(ownerID) != nil
}

func (e governanceClaimInstantsWorldEnv) AuditClaimEvent(nowTick uint64, actorID string, action string, pos Vec3i, reason string, details map[string]any) {
	if e.w == nil {
		return
	}
	e.w.auditEvent(nowTick, actorID, action, pos, reason, details)
}

func (e economyInstantsWorldEnv) ResolveTradeTax(tr *Trade, from *Agent, to *Agent, nowTick uint64) economyinstantspkg.TradeTaxResolution {
	if e.w == nil || tr == nil || from == nil || to == nil {
		return economyinstantspkg.TradeTaxResolution{}
	}
	landFrom, _ := e.w.permissionsFor(from.ID, from.Pos)
	landTo, _ := e.w.permissionsFor(to.ID, to.Pos)
	res := economyinstantspkg.TradeTaxResolution{}
	if landFrom != nil && landTo != nil {
		res.Rate = taxpkg.EffectiveMarketTax(landFrom.MarketTax, landFrom.LandID == landTo.LandID, e.w.activeEventID, nowTick, e.w.activeEventEnds)
	}
	if res.Rate <= 0 || landFrom == nil || landFrom.Owner == "" {
		return res
	}
	if owner := e.w.agents[landFrom.Owner]; owner != nil {
		res.Sink = owner.Inventory
	} else if org := e.w.orgByID(landFrom.Owner); org != nil {
		res.Sink = e.w.orgTreasury(org)
	}
	res.LandID = landFrom.LandID
	res.TaxTo = landFrom.Owner
	return res
}

func handleInstantSay(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	sessioninstantspkg.HandleSay(
		sessionInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
		w.cfg.AllowTrade,
		sessioninstantspkg.SayRateLimits{
			SayWindowTicks:       w.cfg.RateLimits.SayWindowTicks,
			SayMax:               w.cfg.RateLimits.SayMax,
			MarketSayWindowTicks: w.cfg.RateLimits.MarketSayWindowTicks,
			MarketSayMax:         w.cfg.RateLimits.MarketSayMax,
		},
	)
}

func handleInstantWhisper(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	sessioninstantspkg.HandleWhisper(
		sessionInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
		sessioninstantspkg.WhisperLimits{
			WindowTicks: w.cfg.RateLimits.WhisperWindowTicks,
			Max:         w.cfg.RateLimits.WhisperMax,
		},
	)
}

func handleInstantEat(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	sessioninstantspkg.HandleEat(actionResult, a, inst, nowTick, w.catalogs.Items.Defs)
}

func handleInstantSaveMemory(_ *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	sessioninstantspkg.HandleSaveMemory(actionResult, a, inst, nowTick)
}

func handleInstantLoadMemory(_ *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	sessioninstantspkg.HandleLoadMemory(actionResult, a, inst, nowTick)
}

func handleInstantOfferTrade(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	economyinstantspkg.HandleOfferTrade(
		economyInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
		w.cfg.AllowTrade,
		w.cfg.RateLimits.OfferTradeWindowTicks,
		w.cfg.RateLimits.OfferTradeMax,
	)
}

func handleInstantAcceptTrade(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	economyinstantspkg.HandleAcceptTrade(
		economyInstantsWorldEnv{w: w},
		actionResult,
		economyinstantspkg.TradeAcceptHooks{
			OnCompleted: func(out economyinstantspkg.TradeAcceptOutcome) {
				w.auditEvent(nowTick, a.ID, "TRADE", Vec3i{}, "ACCEPT_TRADE", map[string]any{
					"trade_id":       out.Trade.TradeID,
					"from":           out.Trade.From,
					"to":             out.Trade.To,
					"offer":          inventorypkg.EncodeItemPairs(out.Trade.Offer),
					"request":        inventorypkg.EncodeItemPairs(out.Trade.Request),
					"value_offer":    out.ValueOffer,
					"value_request":  out.ValueReq,
					"mutual_benefit": out.MutualOK,
					"tax_rate":       out.Tax.Rate,
					"tax_paid_off":   inventorypkg.EncodeItemPairs(out.TaxPaidOff),
					"tax_paid_req":   inventorypkg.EncodeItemPairs(out.TaxPaidReq),
					"land_id":        out.Tax.LandID,
					"tax_to":         out.Tax.TaxTo,
				})
				w.bumpRepTrade(out.From.ID, 2)
				w.bumpRepTrade(out.To.ID, 2)
				if out.MutualOK {
					w.bumpRepSocial(out.From.ID, 1)
					w.bumpRepSocial(out.To.ID, 1)
				}
				if w.stats != nil {
					w.stats.RecordTrade(nowTick)
				}
				if !out.MutualOK {
					return
				}
				w.funOnTrade(out.From, nowTick)
				w.funOnTrade(out.To, nowTick)
				if w.activeEventID == "MARKET_WEEK" && nowTick < w.activeEventEnds {
					w.funOnWorldEventParticipation(out.From, w.activeEventID, nowTick)
					w.funOnWorldEventParticipation(out.To, w.activeEventID, nowTick)
					w.addFun(out.From, nowTick, "NARRATIVE", "market_week_trade", out.From.FunDecayDelta("narrative:market_week_trade", 5, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
					w.addFun(out.To, nowTick, "NARRATIVE", "market_week_trade", out.To.FunDecayDelta("narrative:market_week_trade", 5, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
					out.From.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "TRADE"})
					out.To.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "TRADE"})
				}
			},
		},
		a,
		inst,
		nowTick,
		w.cfg.AllowTrade,
	)
}

func handleInstantDeclineTrade(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	economyinstantspkg.HandleDeclineTrade(
		economyInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
		w.cfg.AllowTrade,
	)
}

func handleInstantPostBoard(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	postingpkg.HandlePostBoard(
		observerPostingWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
		postingpkg.PostingRateLimits{
			PostWindowTicks: w.cfg.RateLimits.PostBoardWindowTicks,
			PostMax:         w.cfg.RateLimits.PostBoardMax,
		},
	)
}

func handleInstantSearchBoard(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	postingpkg.HandleSearchBoard(
		observerPostingWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantSetSign(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	postingpkg.HandleSetSign(
		observerPostingWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
	)
}

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
		term.Reserve(item, n)
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
			ctx := contractinstantspkg.BuildTerminalContext(
				true,
				term.Type,
				contractinstantspkg.Vec3{X: term.Pos.X, Y: term.Pos.Y, Z: term.Pos.Z},
				contractinstantspkg.Vec3{X: c.TerminalPos.X, Y: c.TerminalPos.Y, Z: c.TerminalPos.Z},
				contractinstantspkg.Vec3{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
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
		term.Reserve(item, n)
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
			ctx := contractinstantspkg.BuildTerminalContext(
				true,
				terminalType,
				contractinstantspkg.Vec3{X: term.Pos.X, Y: term.Pos.Y, Z: term.Pos.Z},
				contractinstantspkg.Vec3{X: c.TerminalPos.X, Y: c.TerminalPos.Y, Z: c.TerminalPos.Z},
				contractinstantspkg.Vec3{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
			)
			terminalMatch = ctx.Matches
			distance = ctx.Distance
			switch c.Kind {
			case "GATHER", "DELIVER":
				requirementsOK = reppkg.HasAvailable(c.Requirements, term.AvailableCount)
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
			term.AddOwed(c.Poster, item, n)
		})
	}
	runtimepkg.PayoutItems(term.Inventory, c.Reward, term.Unreserve, func(item string, n int) {
		a.Inventory[item] += n
	})
	runtimepkg.PayoutItems(term.Inventory, c.Deposit, term.Unreserve, func(item string, n int) {
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

func handleInstantToggleSwitch(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	conveyorinstantspkg.HandleToggleSwitch(
		conveyorInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantClaimOwed(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	contractinstantspkg.HandleClaimOwed(
		contractInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantSetPermissions(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleSetPermissions(
		governanceClaimInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantUpgradeClaim(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleUpgradeClaim(
		governanceClaimInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantAddMember(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleAddMember(
		governanceClaimInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantRemoveMember(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleRemoveMember(
		governanceClaimInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantDeedLand(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleDeedLand(
		governanceClaimInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
	)
}
func handleInstantProposeLaw(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := lawspkg.ValidateProposeInput(w.cfg.AllowLaws, inst.LandID, inst.TemplateID, inst.Params); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	land := w.claims[inst.LandID]
	if land == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !w.isLandMember(a.ID, land) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not eligible"))
		return
	}
	if _, ok := w.catalogs.Laws.ByID[inst.TemplateID]; !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "unknown law template"))
		return
	}

	params, err := lawspkg.NormalizeLawParams(inst.TemplateID, inst.Params, func(item string) bool {
		_, ok := w.catalogs.Items.Defs[item]
		return ok
	})
	if err != nil {
		if errors.Is(err, lawspkg.ErrUnsupportedLawTemplate) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "unsupported template"))
			return
		}
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
		return
	}

	tmpl := w.catalogs.Laws.ByID[inst.TemplateID]
	title := lawspkg.ResolveLawTitle(inst.Title, tmpl.Title)
	lawID := w.newLawID()
	timeline := lawspkg.BuildLawTimeline(nowTick, w.cfg.LawNoticeTicks, w.cfg.LawVoteTicks)
	law := &Law{
		LawID:          lawID,
		LandID:         land.LandID,
		TemplateID:     inst.TemplateID,
		Title:          title,
		Params:         params,
		ProposedBy:     a.ID,
		ProposedTick:   nowTick,
		NoticeEndsTick: timeline.NoticeEnds,
		VoteEndsTick:   timeline.VoteEnds,
		Status:         LawNotice,
		Votes:          map[string]string{},
	}
	w.laws[lawID] = law
	w.broadcastLawEvent(nowTick, "PROPOSED", law, "")
	w.auditEvent(nowTick, a.ID, "LAW_PROPOSE", land.Anchor, "PROPOSE_LAW", map[string]any{
		"law_id":        lawID,
		"land_id":       land.LandID,
		"template_id":   inst.TemplateID,
		"title":         title,
		"notice_ends":   law.NoticeEndsTick,
		"vote_ends":     law.VoteEndsTick,
		"params":        law.Params,
		"proposed_by":   a.ID,
		"proposed_tick": nowTick,
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "law_id": lawID})
	if w.activeEventID == "CIVIC_VOTE" && nowTick < w.activeEventEnds {
		w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
		w.addFun(a, nowTick, "NARRATIVE", "civic_vote_propose", a.FunDecayDelta("narrative:civic_vote_propose", 6, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
		a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "PROPOSE_LAW", "law_id": lawID})
	}
}

func handleInstantVote(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := lawspkg.ValidateVoteInput(w.cfg.AllowLaws, inst.LawID, inst.Choice); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	law := w.laws[inst.LawID]
	if law == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "law not found"))
		return
	}
	if law.Status != LawVoting {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "law not in voting"))
		return
	}
	land := w.claims[law.LandID]
	if land == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !w.isLandMember(a.ID, land) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not eligible to vote"))
		return
	}
	choice := lawspkg.NormalizeVoteChoice(inst.Choice)
	if choice == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad choice"))
		return
	}
	if law.Votes == nil {
		law.Votes = map[string]string{}
	}
	law.Votes[a.ID] = choice
	w.funOnVote(a, nowTick)
	w.auditEvent(nowTick, a.ID, "LAW_VOTE", land.Anchor, "VOTE", map[string]any{
		"law_id":   law.LawID,
		"land_id":  law.LandID,
		"choice":   choice,
		"voter_id": a.ID,
	})
	if w.activeEventID == "CIVIC_VOTE" && nowTick < w.activeEventEnds {
		a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "VOTE", "law_id": law.LawID})
	}
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantCreateOrg(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleCreateOrg(
		governanceOrgInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantJoinOrg(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleJoinOrg(
		governanceOrgInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantOrgDeposit(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleOrgDeposit(
		governanceOrgInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantOrgWithdraw(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleOrgWithdraw(
		governanceOrgInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantLeaveOrg(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleLeaveOrg(
		governanceOrgInstantsWorldEnv{w: w},
		actionResult,
		a,
		inst,
		nowTick,
	)
}
