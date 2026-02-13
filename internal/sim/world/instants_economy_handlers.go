package world

import (
	"voxelcraft.ai/internal/protocol"
	economyinstantspkg "voxelcraft.ai/internal/sim/world/feature/economy/instants"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
)

func handleInstantOfferTrade(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	economyinstantspkg.HandleOfferTrade(
		newEconomyInstantsEnv(w),
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
		newEconomyInstantsEnv(w),
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
		newEconomyInstantsEnv(w),
		actionResult,
		a,
		inst,
		nowTick,
		w.cfg.AllowTrade,
	)
}
