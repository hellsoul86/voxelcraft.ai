package world

import (
	economyinstantspkg "voxelcraft.ai/internal/sim/world/feature/economy/instants"
	taxpkg "voxelcraft.ai/internal/sim/world/feature/economy/tax"
)

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
