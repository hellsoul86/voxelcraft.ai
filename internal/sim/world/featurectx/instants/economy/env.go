package economy

import economyinstantspkg "voxelcraft.ai/internal/sim/world/feature/economy/instants"
import modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"

type Env struct {
	PermissionsForFn  func(agentID string, pos modelpkg.Vec3i) map[string]bool
	AgentByIDFn       func(agentID string) *modelpkg.Agent
	NewTradeIDFn      func() string
	PutTradeFn        func(tr *modelpkg.Trade)
	GetTradeFn        func(tradeID string) *modelpkg.Trade
	DeleteTradeFn     func(tradeID string)
	ResolveTradeTaxFn func(tr *modelpkg.Trade, from *modelpkg.Agent, to *modelpkg.Agent, nowTick uint64) economyinstantspkg.TradeTaxResolution
}

func (e Env) PermissionsFor(agentID string, pos modelpkg.Vec3i) map[string]bool {
	if e.PermissionsForFn == nil {
		return map[string]bool{}
	}
	return e.PermissionsForFn(agentID, pos)
}

func (e Env) AgentByID(agentID string) *modelpkg.Agent {
	if e.AgentByIDFn == nil {
		return nil
	}
	return e.AgentByIDFn(agentID)
}

func (e Env) NewTradeID() string {
	if e.NewTradeIDFn == nil {
		return ""
	}
	return e.NewTradeIDFn()
}

func (e Env) PutTrade(tr *modelpkg.Trade) {
	if e.PutTradeFn != nil {
		e.PutTradeFn(tr)
	}
}

func (e Env) GetTrade(tradeID string) *modelpkg.Trade {
	if e.GetTradeFn == nil {
		return nil
	}
	return e.GetTradeFn(tradeID)
}

func (e Env) DeleteTrade(tradeID string) {
	if e.DeleteTradeFn != nil {
		e.DeleteTradeFn(tradeID)
	}
}

func (e Env) ResolveTradeTax(tr *modelpkg.Trade, from *modelpkg.Agent, to *modelpkg.Agent, nowTick uint64) economyinstantspkg.TradeTaxResolution {
	if e.ResolveTradeTaxFn == nil {
		return economyinstantspkg.TradeTaxResolution{}
	}
	return e.ResolveTradeTaxFn(tr, from, to, nowTick)
}
