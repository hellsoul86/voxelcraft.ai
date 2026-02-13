package world

import (
	"voxelcraft.ai/internal/protocol"
	runtimepkg "voxelcraft.ai/internal/sim/world/feature/movement/runtime"
)

func (w *World) systemMovementImpl(nowTick uint64) {
	runtimepkg.RunMovementSystem(newMovementTaskEnv(w), runtimepkg.SystemInput{
		NowTick:           nowTick,
		Weather:           w.weather,
		ActiveEventID:     w.activeEventID,
		ActiveEventRadius: w.activeEventRadius,
		ActiveEventEnds:   w.activeEventEnds,
		ActiveEventCenter: w.activeEventCenter,
	})
}

func handleTaskStop(_ *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	a.MoveTask = nil
	a.AddEvent(actionResult(nowTick, tr.ID, true, "", "stopped"))
}

func handleTaskMoveTo(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	runtimepkg.HandleTaskMoveTo(newMovementTaskEnv(w), actionResult, a, tr, nowTick)
}

func handleTaskFollow(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	runtimepkg.HandleTaskFollow(newMovementTaskEnv(w), actionResult, a, tr, nowTick)
}
