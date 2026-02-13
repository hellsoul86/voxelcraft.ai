package world

import (
	"voxelcraft.ai/internal/protocol"
	postingpkg "voxelcraft.ai/internal/sim/world/feature/observer/posting"
)

func handleInstantPostBoard(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	postingpkg.HandlePostBoard(
		newObserverPostingEnv(w),
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
		newObserverPostingEnv(w),
		actionResult,
		a,
		inst,
		nowTick,
	)
}

func handleInstantSetSign(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	postingpkg.HandleSetSign(
		newObserverPostingEnv(w),
		actionResult,
		a,
		inst,
		nowTick,
	)
}
