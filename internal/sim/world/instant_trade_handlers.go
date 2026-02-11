package world

import "voxelcraft.ai/internal/protocol"

func handleInstantOfferTrade(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if !w.cfg.AllowTrade {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "trade disabled in this world"))
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
	if inst.To == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing to"))
		return
	}
	to := w.agents[inst.To]
	if to == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "target not found"))
		return
	}
	offer, err := parseItemPairs(inst.Offer)
	if err != nil || len(offer) == 0 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad offer"))
		return
	}
	req, err := parseItemPairs(inst.Request)
	if err != nil || len(req) == 0 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad request"))
		return
	}

	tradeID := w.newTradeID()
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
		"offer":    encodeItemPairs(offer),
		"request":  encodeItemPairs(req),
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "trade_id": tradeID})
}

func handleInstantAcceptTrade(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if !w.cfg.AllowTrade {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "trade disabled in this world"))
		return
	}
	if inst.TradeID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing trade_id"))
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
	if !hasItems(from.Inventory, tr.Offer) || !hasItems(a.Inventory, tr.Request) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "missing items"))
		return
	}
	taxRate := 0.0
	var taxSink map[string]int
	if landFrom != nil && landTo != nil && landFrom.LandID == landTo.LandID && landFrom.MarketTax > 0 {
		taxRate = landFrom.MarketTax
		if landFrom.Owner != "" {
			if owner := w.agents[landFrom.Owner]; owner != nil {
				taxSink = owner.Inventory
			} else if org := w.orgByID(landFrom.Owner); org != nil {
				taxSink = w.orgTreasury(org)
			}
		}
	}
	// Event: Market Week temporarily reduces market tax.
	if taxRate > 0 && w.activeEventID == "MARKET_WEEK" && nowTick < w.activeEventEnds {
		taxRate *= 0.5
	}
	applyTransferWithTax(from.Inventory, a.Inventory, tr.Offer, taxSink, taxRate)
	applyTransferWithTax(a.Inventory, from.Inventory, tr.Request, taxSink, taxRate)
	delete(w.trades, inst.TradeID)

	mutualOK, vOffer, vReq := w.tradeMutualBenefit(tr.Offer, tr.Request)
	w.auditEvent(nowTick, a.ID, "TRADE", Vec3i{}, "ACCEPT_TRADE", map[string]any{
		"trade_id":       tr.TradeID,
		"from":           tr.From,
		"to":             tr.To,
		"offer":          encodeItemPairs(tr.Offer),
		"request":        encodeItemPairs(tr.Request),
		"value_offer":    vOffer,
		"value_request":  vReq,
		"mutual_benefit": mutualOK,
		"tax_rate":       taxRate,
		"tax_paid_off":   encodeItemPairs(calcTax(tr.Offer, taxRate)),
		"tax_paid_req":   encodeItemPairs(calcTax(tr.Request, taxRate)),
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
			w.addFun(from, nowTick, "NARRATIVE", "market_week_trade", w.funDecay(from, "narrative:market_week_trade", 5, nowTick))
			w.addFun(a, nowTick, "NARRATIVE", "market_week_trade", w.funDecay(a, "narrative:market_week_trade", 5, nowTick))
			from.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "TRADE"})
			a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "TRADE"})
		}
	}

	from.AddEvent(protocol.Event{"t": nowTick, "type": "TRADE_DONE", "trade_id": tr.TradeID, "with": a.ID})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TRADE_DONE", "trade_id": tr.TradeID, "with": from.ID})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantDeclineTrade(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if !w.cfg.AllowTrade {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "trade disabled in this world"))
		return
	}
	if inst.TradeID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing trade_id"))
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
