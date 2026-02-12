package world

import (
	"errors"
	"fmt"
	"strings"

	"voxelcraft.ai/internal/protocol"
	auditpkg "voxelcraft.ai/internal/sim/world/feature/contracts/audit"
	contractinstantspkg "voxelcraft.ai/internal/sim/world/feature/contracts/instants"
	lifecyclepkg "voxelcraft.ai/internal/sim/world/feature/contracts/lifecycle"
	reppkg "voxelcraft.ai/internal/sim/world/feature/contracts/reputation"
	runtimepkg "voxelcraft.ai/internal/sim/world/feature/contracts/runtime"
	validationpkg "voxelcraft.ai/internal/sim/world/feature/contracts/validation"
	economyinstantspkg "voxelcraft.ai/internal/sim/world/feature/economy/instants"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	taxpkg "voxelcraft.ai/internal/sim/world/feature/economy/tax"
	tradepkg "voxelcraft.ai/internal/sim/world/feature/economy/trade"
	valuepkg "voxelcraft.ai/internal/sim/world/feature/economy/value"
	claimspkg "voxelcraft.ai/internal/sim/world/feature/governance/claims"
	governanceinstantspkg "voxelcraft.ai/internal/sim/world/feature/governance/instants"
	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
	orgspkg "voxelcraft.ai/internal/sim/world/feature/governance/orgs"
	postingpkg "voxelcraft.ai/internal/sim/world/feature/observer/posting"
	searchpkg "voxelcraft.ai/internal/sim/world/feature/observer/search"
	targetspkg "voxelcraft.ai/internal/sim/world/feature/observer/targets"
	chatpkg "voxelcraft.ai/internal/sim/world/feature/session/chat"
	eatpkg "voxelcraft.ai/internal/sim/world/feature/session/eat"
	sessioninstantspkg "voxelcraft.ai/internal/sim/world/feature/session/instants"
	memorypkg "voxelcraft.ai/internal/sim/world/feature/session/memory"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
)

type instantHandler func(*World, *Agent, protocol.InstantReq, uint64)

var instantDispatch = map[string]instantHandler{
	InstantTypeSay:     handleInstantSay,
	InstantTypeWhisper: handleInstantWhisper,
	InstantTypeEat:     handleInstantEat,
	InstantTypeSaveMemory: func(_ *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
		handleInstantSaveMemory(a, inst, nowTick)
	},
	InstantTypeLoadMemory: func(_ *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
		handleInstantLoadMemory(a, inst, nowTick)
	},
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
	if ok, code, msg := sessioninstantspkg.ValidateSayInput(inst.Text); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	ch, ok := chatpkg.NormalizeChatChannel(inst.Channel)
	if !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "invalid channel"))
		return
	}
	if ch == "CITY" {
		if a.OrgID == "" || !w.isOrgMember(a.ID, a.OrgID) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not in org"))
			return
		}
	}
	if ch == "MARKET" {
		if !w.cfg.AllowTrade {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "market disabled in this world"))
			return
		}
		if _, perms := w.permissionsFor(a.ID, a.Pos); !perms["can_trade"] {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "market chat not allowed here"))
			return
		}
	}

	rl := chatpkg.LimitSpec(ch, chatpkg.RateLimits{
		SayWindowTicks:       uint64(w.cfg.RateLimits.SayWindowTicks),
		SayMax:               w.cfg.RateLimits.SayMax,
		MarketSayWindowTicks: uint64(w.cfg.RateLimits.MarketSayWindowTicks),
		MarketSayMax:         w.cfg.RateLimits.MarketSayMax,
	})
	if ok, cd := a.RateLimitAllow(rl.Kind, nowTick, rl.Window, rl.Max); !ok {
		ev := actionResult(nowTick, inst.ID, false, "E_RATE_LIMIT", rl.RateErrMsg)
		ev["cooldown_ticks"] = cd
		ev["cooldown_until_tick"] = nowTick + cd
		a.AddEvent(ev)
		return
	}

	w.broadcastChat(nowTick, a, ch, inst.Text)
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantWhisper(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, cd := a.RateLimitAllow("WHISPER", nowTick, uint64(w.cfg.RateLimits.WhisperWindowTicks), w.cfg.RateLimits.WhisperMax); !ok {
		ev := actionResult(nowTick, inst.ID, false, "E_RATE_LIMIT", "too many WHISPER")
		ev["cooldown_ticks"] = cd
		ev["cooldown_until_tick"] = nowTick + cd
		a.AddEvent(ev)
		return
	}
	if ok, code, msg := sessioninstantspkg.ValidateWhisperInput(inst.To, inst.Text); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	to := w.agents[inst.To]
	if to == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "target not found"))
		return
	}
	to.AddEvent(protocol.Event{
		"t":       nowTick,
		"type":    "CHAT",
		"from":    a.ID,
		"channel": "WHISPER",
		"text":    inst.Text,
	})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantEat(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.ItemID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing item_id"))
		return
	}
	n := eatpkg.NormalizeConsumeCount(inst.Count)
	def, ok := w.catalogs.Items.Defs[inst.ItemID]
	if !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "unknown item"))
		return
	}
	if !eatpkg.IsFood(def.Kind, def.EdibleHP) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "item not edible"))
		return
	}
	if a.Inventory[inst.ItemID] < n {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "missing food"))
		return
	}
	for i := 0; i < n; i++ {
		a.Inventory[inst.ItemID]--
		if a.Inventory[inst.ItemID] <= 0 {
			delete(a.Inventory, inst.ItemID)
		}
	}
	next := eatpkg.ApplyFood(eatpkg.State{
		HP:           a.HP,
		Hunger:       a.Hunger,
		StaminaMilli: a.StaminaMilli,
	}, def.EdibleHP, n)
	a.HP = next.HP
	a.Hunger = next.Hunger
	a.StaminaMilli = next.StaminaMilli
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantSaveMemory(a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := sessioninstantspkg.ValidateSaveMemoryInput(inst.Key); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	mem := map[string]string{}
	for k, v := range a.Memory {
		mem[k] = v.Value
	}
	// Enforce a very small budget (64KB total).
	if memorypkg.OverMemoryBudget(mem, inst.Key, inst.Value, 64*1024) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "memory budget exceeded"))
		return
	}
	a.MemorySave(inst.Key, inst.Value, inst.TTLTicks, nowTick)
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantLoadMemory(a *Agent, inst protocol.InstantReq, nowTick uint64) {
	kvs := a.MemoryLoad(inst.Prefix, inst.Limit, nowTick)
	a.PendingMemory = kvs
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", fmt.Sprintf("loaded %d keys", len(kvs))))
}

func handleInstantOfferTrade(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := economyinstantspkg.ValidateOfferTradeInput(w.cfg.AllowTrade, inst.To); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	if ok, cd := a.RateLimitAllow("OFFER_TRADE", nowTick, uint64(w.cfg.RateLimits.OfferTradeWindowTicks), w.cfg.RateLimits.OfferTradeMax); !ok {
		ev := actionResult(nowTick, inst.ID, false, "E_RATE_LIMIT", "too many OFFER_TRADE")
		ev["cooldown_ticks"] = cd
		ev["cooldown_until_tick"] = nowTick + cd
		a.AddEvent(ev)
		return
	}
	if _, perms := w.permissionsFor(a.ID, a.Pos); !perms["can_trade"] {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "trade not allowed here"))
		return
	}
	to := w.agents[inst.To]
	if to == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "target not found"))
		return
	}
	offer, offerErr := inventorypkg.ParseItemPairs(inst.Offer)
	req, reqErr := inventorypkg.ParseItemPairs(inst.Request)
	if ok, code, msg := economyinstantspkg.ValidateTradeOfferPairs(offer, offerErr, req, reqErr); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}

	tradeID := tradepkg.TradeID(w.nextTradeNum.Add(1))
	w.trades[tradeID] = &Trade{
		TradeID:     tradeID,
		From:        a.ID,
		To:          to.ID,
		Offer:       offer,
		Request:     req,
		CreatedTick: nowTick,
	}
	to.AddEvent(protocol.Event{
		"t":        nowTick,
		"type":     "TRADE_OFFER",
		"trade_id": tradeID,
		"from":     a.ID,
		"offer":    inventorypkg.EncodeItemPairs(offer),
		"request":  inventorypkg.EncodeItemPairs(req),
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "trade_id": tradeID})
}

func handleInstantAcceptTrade(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := economyinstantspkg.ValidateTradeLifecycleInput(w.cfg.AllowTrade, inst.TradeID); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	tr := w.trades[inst.TradeID]
	if tr == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "trade not found"))
		return
	}
	if tr.To != a.ID {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not your trade"))
		return
	}
	from := w.agents[tr.From]
	if from == nil {
		delete(w.trades, inst.TradeID)
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "trader offline"))
		return
	}
	landFrom, permsFrom := w.permissionsFor(from.ID, from.Pos)
	landTo, permsTo := w.permissionsFor(a.ID, a.Pos)
	if !permsFrom["can_trade"] || !permsTo["can_trade"] {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "trade not allowed here"))
		return
	}
	if !inventorypkg.HasItems(from.Inventory, tr.Offer) || !inventorypkg.HasItems(a.Inventory, tr.Request) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "missing items"))
		return
	}
	taxRate := 0.0
	var taxSink map[string]int
	if landFrom != nil && landTo != nil {
		taxRate = taxpkg.EffectiveMarketTax(landFrom.MarketTax, landFrom.LandID == landTo.LandID, w.activeEventID, nowTick, w.activeEventEnds)
	}
	if taxRate > 0 {
		if landFrom.Owner != "" {
			if owner := w.agents[landFrom.Owner]; owner != nil {
				taxSink = owner.Inventory
			} else if org := w.orgByID(landFrom.Owner); org != nil {
				taxSink = w.orgTreasury(org)
			}
		}
	}
	inventorypkg.ApplyTransferWithTax(from.Inventory, a.Inventory, tr.Offer, taxSink, taxRate)
	inventorypkg.ApplyTransferWithTax(a.Inventory, from.Inventory, tr.Request, taxSink, taxRate)
	delete(w.trades, inst.TradeID)

	vOffer := valuepkg.TradeValue(tr.Offer, valuepkg.ItemTradeValue)
	vReq := valuepkg.TradeValue(tr.Request, valuepkg.ItemTradeValue)
	mutualOK := valuepkg.TradeMutualBenefit(vOffer, vReq)
	w.auditEvent(nowTick, a.ID, "TRADE", Vec3i{}, "ACCEPT_TRADE", map[string]any{
		"trade_id":       tr.TradeID,
		"from":           tr.From,
		"to":             tr.To,
		"offer":          inventorypkg.EncodeItemPairs(tr.Offer),
		"request":        inventorypkg.EncodeItemPairs(tr.Request),
		"value_offer":    vOffer,
		"value_request":  vReq,
		"mutual_benefit": mutualOK,
		"tax_rate":       taxRate,
		"tax_paid_off":   inventorypkg.EncodeItemPairs(inventorypkg.CalcTax(tr.Offer, taxRate)),
		"tax_paid_req":   inventorypkg.EncodeItemPairs(inventorypkg.CalcTax(tr.Request, taxRate)),
		"land_id": func() string {
			if landFrom != nil {
				return landFrom.LandID
			}
			return ""
		}(),
		"tax_to": func() string {
			if landFrom != nil {
				return landFrom.Owner
			}
			return ""
		}(),
	})

	// Reputation: successful trade increases trade/social credit.
	w.bumpRepTrade(from.ID, 2)
	w.bumpRepTrade(a.ID, 2)
	if mutualOK {
		w.bumpRepSocial(from.ID, 1)
		w.bumpRepSocial(a.ID, 1)
	}
	if w.stats != nil {
		w.stats.RecordTrade(nowTick)
	}
	if mutualOK {
		w.funOnTrade(from, nowTick)
		w.funOnTrade(a, nowTick)
		if w.activeEventID == "MARKET_WEEK" && nowTick < w.activeEventEnds {
			w.funOnWorldEventParticipation(from, w.activeEventID, nowTick)
			w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
			w.addFun(from, nowTick, "NARRATIVE", "market_week_trade", from.FunDecayDelta("narrative:market_week_trade", 5, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
			w.addFun(a, nowTick, "NARRATIVE", "market_week_trade", a.FunDecayDelta("narrative:market_week_trade", 5, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
			from.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "TRADE"})
			a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "TRADE"})
		}
	}

	from.AddEvent(protocol.Event{"t": nowTick, "type": "TRADE_DONE", "trade_id": tr.TradeID, "with": a.ID})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TRADE_DONE", "trade_id": tr.TradeID, "with": from.ID})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantDeclineTrade(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := economyinstantspkg.ValidateTradeLifecycleInput(w.cfg.AllowTrade, inst.TradeID); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	tr := w.trades[inst.TradeID]
	if tr == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "trade not found"))
		return
	}
	if tr.To != a.ID {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not your trade"))
		return
	}
	from := w.agents[tr.From]
	delete(w.trades, inst.TradeID)
	if from != nil {
		from.AddEvent(protocol.Event{"t": nowTick, "type": "TRADE_DECLINED", "trade_id": tr.TradeID, "by": a.ID})
	}
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "declined"))
}

func handleInstantPostBoard(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, cd := a.RateLimitAllow("POST_BOARD", nowTick, uint64(w.cfg.RateLimits.PostBoardWindowTicks), w.cfg.RateLimits.PostBoardMax); !ok {
		ev := actionResult(nowTick, inst.ID, false, "E_RATE_LIMIT", "too many POST_BOARD")
		ev["cooldown_ticks"] = cd
		ev["cooldown_until_tick"] = nowTick + cd
		a.AddEvent(ev)
		return
	}
	boardID := postingpkg.ResolveBoardID(inst.BoardID, inst.TargetID)
	if ok, code, message := postingpkg.ValidatePostInput(boardID, inst.Title, inst.Body); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, message))
		return
	}

	// Physical bulletin boards are addressed by id "BULLETIN_BOARD@x,y,z" and require proximity.
	physical := false
	var postPos Vec3i
	if typ, pos, ok := parseContainerID(boardID); ok {
		// Posting in claimed land may be restricted by allow_trade for visitors.
		postingAllowed := true
		if land := w.landAt(pos); land != nil && !w.isLandMember(a.ID, land) && !land.Flags.AllowTrade {
			postingAllowed = false
		}
		if ok, code, message := targetspkg.ValidatePhysicalBoardTarget(
			typ,
			w.blockName(w.chunks.GetBlock(pos)),
			Manhattan(a.Pos, pos),
			postingAllowed,
		); !ok {
			a.AddEvent(actionResult(nowTick, inst.ID, false, code, message))
			return
		}
		physical = true
		postPos = pos
		boardID = boardIDAt(pos) // canonicalize
	}

	b := w.boards[boardID]
	if b == nil {
		if physical {
			b = w.ensureBoard(postPos)
		} else {
			b = &Board{BoardID: boardID}
			w.boards[boardID] = b
		}
	}
	postID := w.newPostID()
	b.Posts = append(b.Posts, BoardPost{
		PostID: postID,
		Author: a.ID,
		Title:  inst.Title,
		Body:   inst.Body,
		Tick:   nowTick,
	})
	w.auditEvent(nowTick, a.ID, "BOARD_POST", postPos, "POST_BOARD", map[string]any{
		"board_id": boardID,
		"post_id":  postID,
		"title":    inst.Title,
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "post_id": postID})
}

func handleInstantSearchBoard(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	boardID := postingpkg.ResolveBoardID(inst.BoardID, inst.TargetID)
	query := strings.TrimSpace(inst.Text)
	if ok, code, message := postingpkg.ValidateSearchInput(boardID, query); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, message))
		return
	}

	limit := searchpkg.NormalizeBoardSearchLimit(inst.Limit)

	// Physical bulletin boards are addressed by id "BULLETIN_BOARD@x,y,z" and require proximity.
	if typ, pos, ok := parseContainerID(boardID); ok && typ == "BULLETIN_BOARD" {
		if ok, code, message := targetspkg.ValidatePhysicalBoardTarget(
			typ,
			w.blockName(w.chunks.GetBlock(pos)),
			Manhattan(a.Pos, pos),
			true,
		); !ok {
			a.AddEvent(actionResult(nowTick, inst.ID, false, code, message))
			return
		}
		boardID = boardIDAt(pos) // canonicalize
		if w.boards[boardID] == nil {
			w.ensureBoard(pos)
		}
	}

	b := w.boards[boardID]
	if b == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "board not found"))
		return
	}

	results := searchpkg.MatchBoardPosts(b.Posts, query, limit)
	a.AddEvent(protocol.Event{
		"t":           nowTick,
		"type":        "BOARD_SEARCH",
		"board_id":    boardID,
		"query":       query,
		"total_posts": len(b.Posts),
		"results":     results,
	})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantSetSign(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	target := strings.TrimSpace(inst.TargetID)
	if target == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	typ, pos, ok := parseContainerID(target)
	if !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "invalid sign target"))
		return
	}
	if ok, code, message := targetspkg.ValidateSetSignTarget(
		typ,
		w.blockName(w.chunks.GetBlock(pos)),
		Manhattan(a.Pos, pos),
		len(inst.Text),
	); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, message))
		return
	}
	if !w.canBuildAt(a.ID, pos, nowTick) {
		w.bumpRepLaw(a.ID, -1)
		if w.stats != nil {
			w.stats.RecordDenied(nowTick)
		}
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "sign edit denied"))
		return
	}

	s := w.ensureSign(pos)
	s.Text = inst.Text
	s.UpdatedTick = nowTick
	s.UpdatedBy = a.ID
	w.auditEvent(nowTick, a.ID, "SIGN_SET", pos, "SET_SIGN", map[string]any{
		"sign_id":     signIDAt(pos),
		"text":        inst.Text,
		"text_length": len(inst.Text),
	})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
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
	owed := c.ClaimOwed(a.ID)
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

func handleInstantSetPermissions(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := claimspkg.ValidateSetPermissionsInput(inst.LandID, inst.Policy); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	land := w.claims[inst.LandID]
	if ok, code, msg := governanceinstantspkg.ValidateLandAdmin(land != nil, w.isLandAdmin(a.ID, land)); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	next := claimspkg.ApplyPolicyFlags(claimspkg.Flags{
		AllowBuild:  land.Flags.AllowBuild,
		AllowBreak:  land.Flags.AllowBreak,
		AllowDamage: land.Flags.AllowDamage,
		AllowTrade:  land.Flags.AllowTrade,
	}, inst.Policy)
	land.Flags.AllowBuild = next.AllowBuild
	land.Flags.AllowBreak = next.AllowBreak
	land.Flags.AllowDamage = next.AllowDamage
	land.Flags.AllowTrade = next.AllowTrade
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantUpgradeClaim(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := claimspkg.ValidateUpgradeInput(inst.LandID, inst.Radius); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	land := w.claims[inst.LandID]
	if ok, code, msg := governanceinstantspkg.ValidateLandAdmin(land != nil, w.isLandAdmin(a.ID, land)); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	if land.MaintenanceStage >= 1 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "land maintenance stage disallows expansion"))
		return
	}
	target := inst.Radius
	if ok, code, msg := claimspkg.ValidateUpgradeRadius(land.Radius, target); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	if w.blockName(w.chunks.GetBlock(land.Anchor)) != "CLAIM_TOTEM" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "claim totem missing"))
		return
	}

	cost := claimspkg.UpgradeCost(land.Radius, target)
	if len(cost) == 0 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "no upgrade needed"))
		return
	}
	for item, n := range cost {
		if a.Inventory[item] < n {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "missing upgrade materials"))
			return
		}
	}

	records := make([]governanceinstantspkg.ClaimRecord, 0, len(w.claims))
	for _, c := range w.claims {
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
	zones := governanceinstantspkg.BuildZones(records)
	if claimspkg.UpgradeOverlaps(land.Anchor.X, land.Anchor.Z, target, land.LandID, zones) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "claim overlaps existing land"))
		return
	}

	for item, n := range cost {
		a.Inventory[item] -= n
		if a.Inventory[item] <= 0 {
			delete(a.Inventory, item)
		}
	}
	from := land.Radius
	land.Radius = target
	w.auditEvent(nowTick, a.ID, "CLAIM_UPGRADE", land.Anchor, "UPGRADE_CLAIM", map[string]any{
		"land_id": inst.LandID,
		"from":    from,
		"to":      target,
		"cost":    inventorypkg.EncodeItemPairs(cost),
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "land_id": inst.LandID, "radius": target})
}

func handleInstantAddMember(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := claimspkg.ValidateMemberMutationInput(inst.LandID, inst.MemberID); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	land := w.claims[inst.LandID]
	if ok, code, msg := governanceinstantspkg.ValidateLandAdmin(land != nil, w.isLandAdmin(a.ID, land)); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	if land.Members == nil {
		land.Members = map[string]bool{}
	}
	land.Members[inst.MemberID] = true
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantRemoveMember(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := claimspkg.ValidateMemberMutationInput(inst.LandID, inst.MemberID); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	land := w.claims[inst.LandID]
	if ok, code, msg := governanceinstantspkg.ValidateLandAdmin(land != nil, w.isLandAdmin(a.ID, land)); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	if land.Members != nil {
		delete(land.Members, inst.MemberID)
	}
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantDeedLand(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := claimspkg.ValidateDeedInput(inst.LandID, inst.NewOwner); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	land := w.claims[inst.LandID]
	if ok, code, msg := governanceinstantspkg.ValidateLandAdmin(land != nil, w.isLandAdmin(a.ID, land)); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	newOwner := claimspkg.NormalizeNewOwner(inst.NewOwner)
	if newOwner == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad new_owner"))
		return
	}
	if w.agents[newOwner] == nil && w.orgByID(newOwner) == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "new owner not found"))
		return
	}
	land.Owner = newOwner
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
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
	kind := orgspkg.NormalizeOrgKind(inst.OrgKind)
	var k OrgKind
	switch kind {
	case orgspkg.KindGuild:
		k = OrgGuild
	case orgspkg.KindCity:
		k = OrgCity
	default:
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad org_kind"))
		return
	}
	if !orgspkg.ValidateOrgName(inst.OrgName) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad org_name"))
		return
	}
	name := orgspkg.NormalizeOrgName(inst.OrgName)
	if a.OrgID != "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "already in org"))
		return
	}
	orgID := w.newOrgID()
	w.orgs[orgID] = &Organization{
		OrgID:       orgID,
		Kind:        k,
		Name:        name,
		CreatedTick: nowTick,
		MetaVersion: 1,
		Members:     map[string]OrgRole{a.ID: OrgLeader},
		Treasury:    map[string]int{},
	}
	a.OrgID = orgID
	w.auditEvent(nowTick, a.ID, "ORG_CREATE", a.Pos, "CREATE_ORG", map[string]any{
		"org_id":   orgID,
		"org_kind": string(k),
		"org_name": name,
		"leader":   a.ID,
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "org_id": orgID})
}

func handleInstantJoinOrg(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.OrgID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing org_id"))
		return
	}
	org := w.orgByID(inst.OrgID)
	if org == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "org not found"))
		return
	}
	if a.OrgID != "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "already in org"))
		return
	}
	if org.Members == nil {
		org.Members = map[string]OrgRole{}
	}
	org.Members[a.ID] = OrgMember
	org.MetaVersion++
	a.OrgID = org.OrgID
	w.auditEvent(nowTick, a.ID, "ORG_JOIN", a.Pos, "JOIN_ORG", map[string]any{
		"org_id":   org.OrgID,
		"member":   a.ID,
		"org_kind": string(org.Kind),
	})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantOrgDeposit(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := orgspkg.ValidateOrgTransferInput(inst.OrgID, inst.ItemID, inst.Count); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	org := w.orgByID(inst.OrgID)
	if org == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "org not found"))
		return
	}
	if !w.isOrgMember(a.ID, org.OrgID) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not org member"))
		return
	}
	if a.Inventory[inst.ItemID] < inst.Count {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "missing items"))
		return
	}
	a.Inventory[inst.ItemID] -= inst.Count
	if a.Inventory[inst.ItemID] <= 0 {
		delete(a.Inventory, inst.ItemID)
	}
	tr := w.orgTreasury(org)
	tr[inst.ItemID] += inst.Count
	w.auditEvent(nowTick, a.ID, "ORG_DEPOSIT", a.Pos, "ORG_DEPOSIT", map[string]any{
		"org_id": org.OrgID,
		"item":   inst.ItemID,
		"count":  inst.Count,
	})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantOrgWithdraw(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := orgspkg.ValidateOrgTransferInput(inst.OrgID, inst.ItemID, inst.Count); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	org := w.orgByID(inst.OrgID)
	if org == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "org not found"))
		return
	}
	if !w.isOrgAdmin(a.ID, org.OrgID) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not org admin"))
		return
	}
	tr := w.orgTreasury(org)
	if tr[inst.ItemID] < inst.Count {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "treasury lacks items"))
		return
	}
	tr[inst.ItemID] -= inst.Count
	if tr[inst.ItemID] <= 0 {
		delete(tr, inst.ItemID)
	}
	a.Inventory[inst.ItemID] += inst.Count
	w.auditEvent(nowTick, a.ID, "ORG_WITHDRAW", a.Pos, "ORG_WITHDRAW", map[string]any{
		"org_id": org.OrgID,
		"item":   inst.ItemID,
		"count":  inst.Count,
	})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantLeaveOrg(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if a.OrgID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "not in org"))
		return
	}
	org := w.orgByID(a.OrgID)
	orgID := a.OrgID
	a.OrgID = ""
	if org == nil || org.Members == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
		return
	}
	role := org.Members[a.ID]
	delete(org.Members, a.ID)
	org.MetaVersion++
	if len(org.Members) == 0 {
		delete(w.orgs, orgID)
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
		return
	}
	if role == OrgLeader {
		memberIDs := make([]string, 0, len(org.Members))
		for aid := range org.Members {
			memberIDs = append(memberIDs, aid)
		}
		best := orgspkg.SelectNextLeader(memberIDs)
		if best != "" {
			org.Members[best] = OrgLeader
			org.MetaVersion++
		}
	}
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}
