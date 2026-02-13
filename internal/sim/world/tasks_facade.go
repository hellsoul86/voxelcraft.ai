package world

import (
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	claimspkg "voxelcraft.ai/internal/sim/world/feature/governance/claims"
	runtimepkg "voxelcraft.ai/internal/sim/world/feature/movement/runtime"
	workruntimepkg "voxelcraft.ai/internal/sim/world/feature/work/runtime"
)

func (w *World) systemMovementImpl(nowTick uint64) {
	runtimepkg.RunMovementSystem(movementTaskReqWorldEnv{w: w}, runtimepkg.SystemInput{
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
	runtimepkg.HandleTaskMoveTo(movementTaskReqWorldEnv{w: w}, actionResult, a, tr, nowTick)
}

func handleTaskFollow(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	runtimepkg.HandleTaskFollow(movementTaskReqWorldEnv{w: w}, actionResult, a, tr, nowTick)
}

func handleTaskMine(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	workruntimepkg.HandleTaskMine(newWorkTaskReqEnv(w), actionResult, a, tr, nowTick, w.cfg.AllowMine)
}

func handleTaskGather(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	workruntimepkg.HandleTaskGather(newWorkTaskReqEnv(w), actionResult, a, tr, nowTick)
}

func handleTaskPlace(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	workruntimepkg.HandleTaskPlace(newWorkTaskReqEnv(w), actionResult, a, tr, nowTick, w.cfg.AllowPlace)
}

func handleTaskOpen(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	workruntimepkg.HandleTaskOpen(newWorkTaskReqEnv(w), actionResult, a, tr, nowTick)
}

func handleTaskTransfer(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	workruntimepkg.HandleTaskTransfer(newWorkTaskReqEnv(w), actionResult, a, tr, nowTick)
}

func handleTaskCraft(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	workruntimepkg.HandleTaskCraft(newWorkTaskReqEnv(w), actionResult, a, tr, nowTick)
}

func handleTaskSmelt(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	workruntimepkg.HandleTaskSmelt(newWorkTaskReqEnv(w), actionResult, a, tr, nowTick)
}

func (w *World) systemWorkImpl(nowTick uint64) {
	for _, a := range w.sortedAgents() {
		wt := a.WorkTask
		if wt == nil {
			continue
		}

		switch wt.Kind {
		case tasks.KindMine:
			w.tickMine(a, wt, nowTick)
		case tasks.KindGather:
			w.tickGather(a, wt, nowTick)
		case tasks.KindPlace:
			w.tickPlace(a, wt, nowTick)
		case tasks.KindOpen:
			w.tickOpen(a, wt, nowTick)
		case tasks.KindTransfer:
			w.tickTransfer(a, wt, nowTick)
		case tasks.KindCraft:
			w.tickCraft(a, wt, nowTick)
		case tasks.KindSmelt:
			w.tickSmelt(a, wt, nowTick)
		case tasks.KindBuildBlueprint:
			w.tickBuildBlueprint(a, wt, nowTick)
		}
	}
}
func (w *World) tickMine(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	workruntimepkg.TickMine(newWorkTaskExecEnv(w), a, wt, nowTick)
}

func (w *World) tickGather(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	workruntimepkg.TickGather(newWorkTaskExecEnv(w), a, wt, nowTick)
}

func (w *World) tickPlace(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	workruntimepkg.TickPlace(newWorkTaskExecEnv(w), a, wt, nowTick)
}

func (w *World) tickOpen(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	workruntimepkg.TickOpen(newWorkTaskExecEnv(w), a, wt, nowTick)
}

func (w *World) tickTransfer(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	workruntimepkg.TickTransfer(newWorkTaskExecEnv(w), a, wt, nowTick)
}

func (w *World) tickCraft(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	workruntimepkg.TickCraft(newWorkTaskExecEnv(w), a, wt, nowTick)
}

func (w *World) tickSmelt(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	workruntimepkg.TickSmelt(newWorkTaskExecEnv(w), a, wt, nowTick)
}

func (w *World) tickBuildBlueprint(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	res := workruntimepkg.TickBuildBlueprint(newWorkTaskExecEnv(w), a, wt, nowTick, w.cfg.BlueprintBlocksPerTick)
	if !res.Completed {
		return
	}
	if w.stats != nil {
		w.stats.RecordBlueprintComplete(nowTick)
	}
	w.registerStructure(nowTick, a.ID, wt.BlueprintID, res.Anchor, res.Rotation)
	w.funOnBlueprintComplete(a, nowTick)
	// Event-specific build bonuses.
	if w.activeEventID != "" && nowTick < w.activeEventEnds {
		switch w.activeEventID {
		case "BUILDER_EXPO":
			w.addFun(a, nowTick, "CREATION", "builder_expo", a.FunDecayDelta("creation:builder_expo", 8, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
			a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "EXPO_BUILD", "blueprint_id": wt.BlueprintID})
		case "BLUEPRINT_FAIR":
			w.addFun(a, nowTick, "INFLUENCE", "blueprint_fair", a.FunDecayDelta("influence:blueprint_fair", 6, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
			a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "FAIR_BUILD", "blueprint_id": wt.BlueprintID})
		}
	}
	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}

func handleTaskClaimLand(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	claimspkg.HandleTaskClaimLand(newClaimTaskEnv(w), actionResult, a, tr, nowTick, w.cfg.AllowClaims)
}

func handleTaskBuildBlueprint(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	workruntimepkg.HandleTaskBuildBlueprint(newWorkTaskReqEnv(w), actionResult, a, tr, nowTick, w.cfg.AllowBuild)
}
