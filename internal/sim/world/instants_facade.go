package world

import (
	"voxelcraft.ai/internal/protocol"
	auditpkg "voxelcraft.ai/internal/sim/world/feature/contracts/audit"
	contractinstantspkg "voxelcraft.ai/internal/sim/world/feature/contracts/instants"
	conveyorinstantspkg "voxelcraft.ai/internal/sim/world/feature/conveyor/runtime"
	economyinstantspkg "voxelcraft.ai/internal/sim/world/feature/economy/instants"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	governanceinstantspkg "voxelcraft.ai/internal/sim/world/feature/governance/instants"
	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
	postingpkg "voxelcraft.ai/internal/sim/world/feature/observer/posting"
	sessioninstantspkg "voxelcraft.ai/internal/sim/world/feature/session/instants"
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
	contractinstantspkg.HandlePostContract(
		contractInstantsWorldEnv{w: w},
		actionResult,
		contractinstantspkg.ContractLifecycleHooks{
			OnPosted: func(out contractinstantspkg.ContractPostOutcome) {
				c := out.Contract
				term := out.Terminal
				w.auditEvent(nowTick, a.ID, "CONTRACT_POST", term.Pos, "POST_CONTRACT", auditpkg.BuildPostAuditFields(
					c.ContractID,
					term.ID(),
					c.Kind,
					inventorypkg.EncodeItemPairs(c.Requirements),
					inventorypkg.EncodeItemPairs(c.Reward),
					inventorypkg.EncodeItemPairs(c.Deposit),
					c.DeadlineTick,
					c.BlueprintID,
					c.Anchor.ToArray(),
					c.Rotation,
				))
			},
		},
		a,
		inst,
		nowTick,
		w.cfg.DayTicks,
	)
}

func handleInstantAcceptContract(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	contractinstantspkg.HandleAcceptContract(
		contractInstantsWorldEnv{w: w},
		actionResult,
		contractinstantspkg.ContractLifecycleHooks{
			OnAccepted: func(out contractinstantspkg.ContractAcceptOutcome) {
				c := out.Contract
				term := out.Terminal
				w.auditEvent(nowTick, a.ID, "CONTRACT_ACCEPT", term.Pos, "ACCEPT_CONTRACT",
					auditpkg.BuildAcceptAuditFields(c.ContractID, term.ID(), c.Kind, c.Poster, c.Acceptor, inventorypkg.EncodeItemPairs(c.Deposit)))
			},
		},
		a,
		inst,
		nowTick,
	)
}

func handleInstantSubmitContract(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	contractinstantspkg.HandleSubmitContract(
		contractInstantsWorldEnv{w: w},
		actionResult,
		contractinstantspkg.ContractLifecycleHooks{
			OnSubmitted: func(out contractinstantspkg.ContractSubmitOutcome) {
				c := out.Contract
				term := out.Terminal
				switch c.Kind {
				case "GATHER", "DELIVER":
					w.addTradeCredit(nowTick, a.ID, c.Poster, c.Kind)
				case "BUILD":
					w.addBuildCredit(nowTick, a.ID, c.Poster, c.Kind)
				}
				w.auditEvent(nowTick, a.ID, "CONTRACT_COMPLETE", term.Pos, "SUBMIT_CONTRACT",
					auditpkg.BuildSubmitAuditFields(c.ContractID, term.ID(), c.Kind, c.Poster, c.Acceptor))
			},
		},
		a,
		inst,
		nowTick,
	)
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
	governanceinstantspkg.HandleProposeLaw(
		governanceLawInstantsWorldEnv{w: w},
		actionResult,
		governanceinstantspkg.LawInstantHooks{
			OnProposed: func(law *lawspkg.Law) {
				if w.activeEventID == "CIVIC_VOTE" && nowTick < w.activeEventEnds {
					w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
					w.addFun(a, nowTick, "NARRATIVE", "civic_vote_propose", a.FunDecayDelta("narrative:civic_vote_propose", 6, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
					a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "PROPOSE_LAW", "law_id": law.LawID})
				}
			},
		},
		a,
		inst,
		nowTick,
		w.cfg.AllowLaws,
		w.cfg.LawNoticeTicks,
		w.cfg.LawVoteTicks,
	)
}

func handleInstantVote(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	governanceinstantspkg.HandleVoteLaw(
		governanceLawInstantsWorldEnv{w: w},
		actionResult,
		governanceinstantspkg.LawInstantHooks{
			OnVoted: func(law *lawspkg.Law, _ string) {
				w.funOnVote(a, nowTick)
				if w.activeEventID == "CIVIC_VOTE" && nowTick < w.activeEventEnds {
					a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "VOTE", "law_id": law.LawID})
				}
			},
		},
		a,
		inst,
		nowTick,
		w.cfg.AllowLaws,
	)
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
