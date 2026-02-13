package world

import (
	"voxelcraft.ai/internal/protocol"
	sessioninstantspkg "voxelcraft.ai/internal/sim/world/feature/session/instants"
)

func handleInstantSay(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	sessioninstantspkg.HandleSay(
		newSessionInstantsEnv(w),
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
		newSessionInstantsEnv(w),
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
