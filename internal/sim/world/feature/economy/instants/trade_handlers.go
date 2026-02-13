package instants

import (
	"fmt"

	"voxelcraft.ai/internal/protocol"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	valuepkg "voxelcraft.ai/internal/sim/world/feature/economy/value"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type ActionResultFn func(tick uint64, ref string, ok bool, code string, message string) protocol.Event

type TradeEnv interface {
	PermissionsFor(agentID string, pos modelpkg.Vec3i) map[string]bool
	AgentByID(agentID string) *modelpkg.Agent
	NewTradeID() string
	PutTrade(tr *modelpkg.Trade)
	GetTrade(tradeID string) *modelpkg.Trade
	DeleteTrade(tradeID string)
}

type TradeTaxResolution struct {
	Rate   float64
	Sink   map[string]int
	LandID string
	TaxTo  string
}

type TradeAcceptOutcome struct {
	Trade       *modelpkg.Trade
	From        *modelpkg.Agent
	To          *modelpkg.Agent
	ValueOffer  int64
	ValueReq    int64
	MutualOK    bool
	Tax         TradeTaxResolution
	TaxPaidOff  map[string]int
	TaxPaidReq  map[string]int
	CompletedAt uint64
}

type TradeAcceptEnv interface {
	GetTrade(tradeID string) *modelpkg.Trade
	DeleteTrade(tradeID string)
	AgentByID(agentID string) *modelpkg.Agent
	PermissionsFor(agentID string, pos modelpkg.Vec3i) map[string]bool
	ResolveTradeTax(tr *modelpkg.Trade, from *modelpkg.Agent, to *modelpkg.Agent, nowTick uint64) TradeTaxResolution
}

type TradeAcceptHooks struct {
	OnCompleted func(TradeAcceptOutcome)
}

func HandleOfferTrade(env TradeEnv, ar ActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64, allowTrade bool, offerTradeWindowTicks int, offerTradeMax int) {
	if ok, code, msg := ValidateOfferTradeInput(allowTrade, inst.To); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	if ok, cd := a.RateLimitAllow("OFFER_TRADE", nowTick, uint64(offerTradeWindowTicks), offerTradeMax); !ok {
		ev := ar(nowTick, inst.ID, false, "E_RATE_LIMIT", "too many OFFER_TRADE")
		ev["cooldown_ticks"] = cd
		ev["cooldown_until_tick"] = nowTick + cd
		a.AddEvent(ev)
		return
	}
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "trade env unavailable"))
		return
	}
	perms := env.PermissionsFor(a.ID, a.Pos)
	if !perms["can_trade"] {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_PERMISSION", "trade not allowed here"))
		return
	}
	to := env.AgentByID(inst.To)
	if to == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "target not found"))
		return
	}
	offer, offerErr := inventorypkg.ParseItemPairs(inst.Offer)
	req, reqErr := inventorypkg.ParseItemPairs(inst.Request)
	if ok, code, msg := ValidateTradeOfferPairs(offer, offerErr, req, reqErr); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}

	tradeID := env.NewTradeID()
	if tradeID == "" {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "trade id allocation failed"))
		return
	}
	env.PutTrade(&modelpkg.Trade{
		TradeID:     tradeID,
		From:        a.ID,
		To:          to.ID,
		Offer:       offer,
		Request:     req,
		CreatedTick: nowTick,
	})
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

func HandleDeclineTrade(env TradeEnv, ar ActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64, allowTrade bool) {
	if ok, code, msg := ValidateTradeLifecycleInput(allowTrade, inst.TradeID); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "trade env unavailable"))
		return
	}
	tr := env.GetTrade(inst.TradeID)
	if tr == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "trade not found"))
		return
	}
	if tr.To != a.ID {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_PERMISSION", "not your trade"))
		return
	}
	from := env.AgentByID(tr.From)
	env.DeleteTrade(inst.TradeID)
	if from != nil {
		from.AddEvent(protocol.Event{"t": nowTick, "type": "TRADE_DECLINED", "trade_id": tr.TradeID, "by": a.ID})
	}
	a.AddEvent(ar(nowTick, inst.ID, true, "", "declined"))
}

func HandleAcceptTrade(env TradeAcceptEnv, ar ActionResultFn, hooks TradeAcceptHooks, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64, allowTrade bool) {
	if ok, code, msg := ValidateTradeLifecycleInput(allowTrade, inst.TradeID); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "trade env unavailable"))
		return
	}
	tr := env.GetTrade(inst.TradeID)
	if tr == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "trade not found"))
		return
	}
	if tr.To != a.ID {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_PERMISSION", "not your trade"))
		return
	}
	from := env.AgentByID(tr.From)
	if from == nil {
		env.DeleteTrade(inst.TradeID)
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "trader offline"))
		return
	}
	permsFrom := env.PermissionsFor(from.ID, from.Pos)
	permsTo := env.PermissionsFor(a.ID, a.Pos)
	if !permsFrom["can_trade"] || !permsTo["can_trade"] {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_PERMISSION", "trade not allowed here"))
		return
	}
	if !inventorypkg.HasItems(from.Inventory, tr.Offer) || !inventorypkg.HasItems(a.Inventory, tr.Request) {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_RESOURCE", "missing items"))
		return
	}

	tax := env.ResolveTradeTax(tr, from, a, nowTick)
	inventorypkg.ApplyTransferWithTax(from.Inventory, a.Inventory, tr.Offer, tax.Sink, tax.Rate)
	inventorypkg.ApplyTransferWithTax(a.Inventory, from.Inventory, tr.Request, tax.Sink, tax.Rate)
	env.DeleteTrade(inst.TradeID)

	vOffer := valuepkg.TradeValue(tr.Offer, valuepkg.ItemTradeValue)
	vReq := valuepkg.TradeValue(tr.Request, valuepkg.ItemTradeValue)
	mutualOK := valuepkg.TradeMutualBenefit(vOffer, vReq)
	if hooks.OnCompleted != nil {
		hooks.OnCompleted(TradeAcceptOutcome{
			Trade:       tr,
			From:        from,
			To:          a,
			ValueOffer:  vOffer,
			ValueReq:    vReq,
			MutualOK:    mutualOK,
			Tax:         tax,
			TaxPaidOff:  inventorypkg.CalcTax(tr.Offer, tax.Rate),
			TaxPaidReq:  inventorypkg.CalcTax(tr.Request, tax.Rate),
			CompletedAt: nowTick,
		})
	}

	from.AddEvent(protocol.Event{"t": nowTick, "type": "TRADE_DONE", "trade_id": tr.TradeID, "with": a.ID})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TRADE_DONE", "trade_id": tr.TradeID, "with": from.ID})
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}

func SimpleTradeIDFromCounter(next uint64) string {
	return fmt.Sprintf("TR%06d", next)
}
