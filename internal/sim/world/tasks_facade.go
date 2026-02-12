package world

import (
	"strings"
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/tasks"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	claimspkg "voxelcraft.ai/internal/sim/world/feature/governance/claims"
	detourpkg "voxelcraft.ai/internal/sim/world/feature/movement/detour"
	paramspkg "voxelcraft.ai/internal/sim/world/feature/movement/params"
	runtimepkg "voxelcraft.ai/internal/sim/world/feature/movement/runtime"
	interactpkg "voxelcraft.ai/internal/sim/world/feature/work/interact"
	limitspkg "voxelcraft.ai/internal/sim/world/feature/work/limits"
	miningpkg "voxelcraft.ai/internal/sim/world/feature/work/mining"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
	"voxelcraft.ai/internal/sim/world/logic/mathx"
)

func (w *World) systemMovementImpl(nowTick uint64) {
	for _, a := range w.sortedAgents() {
		mt := a.MoveTask
		if mt == nil {
			continue
		}
		var target Vec3i
		switch mt.Kind {
		case tasks.KindMoveTo:
			target = v3FromTask(mt.Target)
			want := runtimepkg.MoveTolerance(mt.Tolerance)
			// Complete when within tolerance; do not teleport to the exact target to avoid skipping obstacles.
			if distXZ(a.Pos, target) <= want {
				w.recordStructureUsage(a.ID, a.Pos, nowTick)
				w.funOnBiome(a, nowTick)
				a.MoveTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": mt.TaskID, "kind": string(mt.Kind)})
				continue
			}

		case tasks.KindFollow:
			t, ok := w.followTargetPos(mt.TargetID)
			if !ok {
				a.MoveTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_INVALID_TARGET", "message": "follow target not found"})
				continue
			}
			mt.Target = v3ToTask(t)
			target = t

			want := runtimepkg.MoveTolerance(mt.Distance)
			if distXZ(a.Pos, target) <= want {
				// Stay close; keep task active until canceled.
				continue
			}

		default:
			continue
		}

		// Storm slows travel but should not deadlock tasks.
		if runtimepkg.ShouldSkipStorm(w.weather, nowTick) {
			continue
		}

		// Event hazard: flood zones slow travel.
		if runtimepkg.ShouldSkipFlood(
			w.activeEventID,
			w.activeEventRadius,
			nowTick,
			w.activeEventEnds,
			runtimepkg.Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
			runtimepkg.Pos{X: w.activeEventCenter.X, Y: w.activeEventCenter.Y, Z: w.activeEventCenter.Z},
		) {
			continue
		}

		// Moving costs stamina; if too tired, wait and recover.
		const moveCost = 8
		if a.StaminaMilli < moveCost {
			continue
		}
		a.StaminaMilli -= moveCost

		// Deterministic 2D stepping with minimal obstacle avoidance:
		// - Pick primary axis by abs(dx)>=abs(dz)
		// - If the next cell on the primary axis is blocked by a solid block, try the secondary axis.
		dx := target.X - a.Pos.X
		dz := target.Z - a.Pos.Z

		primaryX := runtimepkg.PrimaryAxis(dx, dz)
		next := a.Pos
		p1 := runtimepkg.PrimaryStep(runtimepkg.Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z}, dx, dz, primaryX)
		next1 := Vec3i{X: p1.X, Y: p1.Y, Z: p1.Z}
		next1.Y = w.surfaceY(next1.X, next1.Z)
		next = next1

		if w.blockSolid(w.chunks.GetBlock(next1)) {
			// Try the secondary axis only when primary step is blocked.
			p2 := runtimepkg.SecondaryStep(runtimepkg.Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z}, dx, dz, primaryX)
			next2 := Vec3i{X: p2.X, Y: p2.Y, Z: p2.Z}
			if next2 != a.Pos {
				next2.Y = w.surfaceY(next2.X, next2.Z)
				if !w.blockSolid(w.chunks.GetBlock(next2)) {
					next = next2
				}
			}
		}

		// If both primary+secondary are blocked, attempt a small deterministic detour so agents
		// don't have to constantly re-issue MOVE_TO on cluttered terrain.
		if w.blockSolid(w.chunks.GetBlock(next)) {
			if alt, ok := detourpkg.DetourStep2D(
				detourpkg.Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
				detourpkg.Pos{X: target.X, Y: target.Y, Z: target.Z},
				16,
				func(p detourpkg.Pos) bool {
					return w.chunks.inBounds(Vec3i{X: p.X, Y: p.Y, Z: p.Z})
				},
				func(p detourpkg.Pos) bool {
					return w.blockSolid(w.chunks.GetBlock(Vec3i{X: p.X, Y: p.Y, Z: p.Z}))
				},
			); ok {
				next = Vec3i{X: alt.X, Y: alt.Y, Z: alt.Z}
			}
		}

		// Reputation consequence: low Law rep agents may be blocked from entering a CITY core area.
		// This is a system-level "wanted" restriction separate from access passes.
		if toLand := w.landAt(next); toLand != nil && w.landCoreContains(toLand, next) && !w.isLandMember(a.ID, toLand) {
			if org := w.orgByID(toLand.Owner); org != nil && org.Kind == OrgCity {
				fromLand := w.landAt(a.Pos)
				entering := fromLand == nil || fromLand.LandID != toLand.LandID || !w.landCoreContains(toLand, a.Pos)
				if entering && a.RepLaw > 0 && a.RepLaw < 200 {
					a.MoveTask = nil
					if w.stats != nil {
						w.stats.RecordDenied(nowTick)
					}
					a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_NO_PERMISSION", "message": "wanted: law reputation too low"})
					continue
				}
			}
		}

		// Land core access pass (law): charge ticket on core entry for non-members.
		if toLand := w.landAt(next); toLand != nil && toLand.AccessPassEnabled && w.landCoreContains(toLand, next) && !w.isLandMember(a.ID, toLand) {
			fromLand := w.landAt(a.Pos)
			entering := fromLand == nil || fromLand.LandID != toLand.LandID || !w.landCoreContains(toLand, a.Pos)
			if entering {
				item := strings.TrimSpace(toLand.AccessTicketItem)
				cost := toLand.AccessTicketCost
				if item == "" || cost <= 0 {
					// Misconfigured law: treat as blocked.
					a.MoveTask = nil
					if w.stats != nil {
						w.stats.RecordDenied(nowTick)
					}
					a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_NO_PERMISSION", "message": "access pass required"})
					continue
				}
				if a.Inventory[item] < cost {
					a.MoveTask = nil
					if w.stats != nil {
						w.stats.RecordDenied(nowTick)
					}
					a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_NO_RESOURCE", "message": "need access ticket"})
					continue
				}
				a.Inventory[item] -= cost
				// Credit to land owner if present (agent or org treasury); else burn.
				if toLand.Owner != "" {
					if owner := w.agents[toLand.Owner]; owner != nil {
						owner.Inventory[item] += cost
					} else if org := w.orgByID(toLand.Owner); org != nil {
						w.orgTreasury(org)[item] += cost
					}
				}
				a.AddEvent(protocol.Event{"t": nowTick, "type": "ACCESS_PASS", "land_id": toLand.LandID, "item": item, "count": cost})
			}
		}

		// Basic collision: treat solid blocks as blocking; allow non-solid (e.g. water/torch/wire).
		if w.blockSolid(w.chunks.GetBlock(Vec3i{X: next.X, Y: next.Y, Z: next.Z})) {
			a.MoveTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_BLOCKED", "message": "blocked"})
			continue
		}
		a.Pos = next
		w.recordStructureUsage(a.ID, a.Pos, nowTick)
		w.funOnBiome(a, nowTick)
	}
}

func handleTaskStop(_ *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	a.MoveTask = nil
	a.AddEvent(actionResult(nowTick, tr.ID, true, "", "stopped"))
}

func handleTaskMoveTo(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.MoveTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "movement task slot occupied"))
		return
	}
	// Reject targets outside the world boundary to avoid agents wandering into the "void"
	// (GetBlock returns AIR outside BoundaryR, and we don't generate chunks there).
	tx, ty, tz := paramspkg.NormalizeTarget(tr.Target[0], tr.Target[2])
	if !w.chunks.inBounds(Vec3i{X: tx, Y: ty, Z: tz}) {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "out of bounds"))
		return
	}
	taskID := w.newTaskID()
	a.MoveTask = &tasks.MovementTask{
		TaskID:      taskID,
		Kind:        tasks.KindMoveTo,
		Target:      tasks.Vec3i{X: tx, Y: ty, Z: tz},
		Tolerance:   tr.Tolerance,
		StartPos:    tasks.Vec3i{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{
		"t":       nowTick,
		"type":    "ACTION_RESULT",
		"ref":     tr.ID,
		"ok":      true,
		"task_id": taskID,
	})
}

func handleTaskFollow(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.MoveTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "movement task slot occupied"))
		return
	}
	if tr.TargetID == "" {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	dist := paramspkg.ClampFollowDistance(tr.Distance)
	target, ok := w.followTargetPos(tr.TargetID)
	if !ok {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "target not found"))
		return
	}
	taskID := w.newTaskID()
	a.MoveTask = &tasks.MovementTask{
		TaskID:      taskID,
		Kind:        tasks.KindFollow,
		Target:      v3ToTask(target),
		TargetID:    tr.TargetID,
		Distance:    dist,
		StartPos:    tasks.Vec3i{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{
		"t":       nowTick,
		"type":    "ACTION_RESULT",
		"ref":     tr.ID,
		"ok":      true,
		"task_id": taskID,
	})
}

func handleTaskMine(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if !w.cfg.AllowMine {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_PERMISSION", "mining disabled in this world"))
		return
	}
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.BlockPos[1] != 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "2D world requires y==0"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindMine,
		BlockPos:    tasks.Vec3i{X: tr.BlockPos[0], Y: 0, Z: tr.BlockPos[2]},
		StartedTick: nowTick,
		WorkTicks:   0,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskGather(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.TargetID == "" {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	if w.items[tr.TargetID] == nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "item entity not found"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindGather,
		TargetID:    tr.TargetID,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskPlace(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if !w.cfg.AllowPlace {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_PERMISSION", "placing disabled in this world"))
		return
	}
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.ItemID == "" {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing item_id"))
		return
	}
	if tr.BlockPos[1] != 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "2D world requires y==0"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindPlace,
		ItemID:      tr.ItemID,
		BlockPos:    tasks.Vec3i{X: tr.BlockPos[0], Y: 0, Z: tr.BlockPos[2]},
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskOpen(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.TargetID == "" {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindOpen,
		TargetID:    tr.TargetID,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskTransfer(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.Src == "" || tr.Dst == "" || tr.ItemID == "" || tr.Count <= 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing src/dst/item_id/count"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:       taskID,
		Kind:         tasks.KindTransfer,
		SrcContainer: tr.Src,
		DstContainer: tr.Dst,
		ItemID:       tr.ItemID,
		Count:        tr.Count,
		StartedTick:  nowTick,
		WorkTicks:    0,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskCraft(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.RecipeID == "" || tr.Count <= 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing recipe_id/count"))
		return
	}
	if _, ok := w.catalogs.Recipes.ByID[tr.RecipeID]; !ok {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "unknown recipe"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindCraft,
		RecipeID:    tr.RecipeID,
		Count:       tr.Count,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskSmelt(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.ItemID == "" || tr.Count <= 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing item_id/count"))
		return
	}
	if _, ok := w.smeltByInput[tr.ItemID]; !ok {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "unsupported smelt item"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindSmelt,
		ItemID:      tr.ItemID,
		Count:       tr.Count,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
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
	pos := v3FromTask(wt.BlockPos)
	// Require within 2 blocks (Manhattan).
	if Manhattan(a.Pos, pos) > 2 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "too far"})
		return
	}
	if !w.canBreakAt(a.ID, pos, nowTick) {
		// Optional law: fine visitors for illegal break attempts (permission-denied only; curfew does not fine).
		if land, perms := w.permissionsFor(a.ID, pos); land != nil && !w.isLandMember(a.ID, land) && !perms["can_break"] &&
			land.FineBreakEnabled && land.FineBreakPerBlock > 0 && strings.TrimSpace(land.FineBreakItem) != "" {
			item := strings.TrimSpace(land.FineBreakItem)
			fine := land.FineBreakPerBlock
			pay := fine
			if have := a.Inventory[item]; have < pay {
				pay = have
			}
			if pay > 0 {
				a.Inventory[item] -= pay
				if land.Owner != "" {
					if owner := w.agents[land.Owner]; owner != nil {
						owner.Inventory[item] += pay
					} else if org := w.orgByID(land.Owner); org != nil {
						w.orgTreasury(org)[item] += pay
					}
				}
				a.AddEvent(protocol.Event{"t": nowTick, "type": "FINE", "land_id": land.LandID, "item": item, "count": pay, "reason": "BREAK_DENIED"})
			}
		}
		a.WorkTask = nil
		w.bumpRepLaw(a.ID, -1)
		if w.stats != nil {
			w.stats.RecordDenied(nowTick)
		}
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "break denied"})
		return
	}
	b := w.chunks.GetBlock(pos)
	if b == w.chunks.gen.Air {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "no block"})
		return
	}
	blockName := w.blockName(b)

	family := miningpkg.MineToolFamilyForBlock(blockName)
	tier := miningpkg.BestToolTier(a.Inventory, family)
	mineWorkNeeded, mineCost := miningpkg.MineParamsForTier(tier)

	// Mining costs stamina; if too tired, wait and recover.
	if a.StaminaMilli < mineCost {
		return
	}
	a.StaminaMilli -= mineCost

	wt.WorkTicks++
	if wt.WorkTicks < mineWorkNeeded {
		return
	}

	// Break block -> AIR, add a very simplified drop.
	// If the block is a container/terminal, handle inventory safely.
	if blockName != "" {
		switch blockName {
		case "CHEST", "FURNACE", "CONTRACT_TERMINAL":
			c := w.containers[pos]
			if c != nil && len(c.Reserved) > 0 {
				// Prevent breaking terminals with escrow-reserved items.
				a.WorkTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "container has reserved items"})
				return
			}
			if c != nil {
				for item, n := range c.Inventory {
					if n > 0 {
						a.Inventory[item] += n
					}
				}
				if c.Owed != nil {
					if owed := c.Owed[a.ID]; owed != nil {
						for item, n := range owed {
							if n > 0 {
								a.Inventory[item] += n
							}
						}
						delete(c.Owed, a.ID)
					}
				}
				w.removeContainer(pos)
			}
		case "BULLETIN_BOARD":
			w.removeBoard(pos)
		case "SIGN":
			w.removeSign(nowTick, a.ID, pos, "MINE")
		case "CONVEYOR":
			w.removeConveyor(nowTick, a.ID, pos, "MINE")
		case "SWITCH":
			w.removeSwitch(nowTick, a.ID, pos, "MINE")
		case "CLAIM_TOTEM":
			w.removeClaimByAnchor(nowTick, a.ID, pos, "MINE")
		}
	}

	w.chunks.SetBlock(pos, w.chunks.gen.Air)
	w.auditSetBlock(nowTick, a.ID, pos, b, w.chunks.gen.Air, "MINE")

	item := w.blockIDToItem(b)
	if item != "" {
		_ = w.spawnItemEntity(nowTick, a.ID, pos, item, 1, "MINE_DROP")
	}
	w.onMinedBlockDuringEvent(a, pos, blockName, nowTick)
	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}

func (w *World) tickGather(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	id := wt.TargetID
	if id == "" {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BAD_REQUEST", "message": "missing target_id"})
		return
	}
	e := w.items[id]
	if e == nil || e.Item == "" || e.Count <= 0 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "item entity not found"})
		return
	}
	if Manhattan(a.Pos, e.Pos) > 2 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "too far"})
		return
	}
	if !w.canPickupItemEntity(a.ID, e.Pos) {
		a.WorkTask = nil
		w.bumpRepLaw(a.ID, -1)
		if w.stats != nil {
			w.stats.RecordDenied(nowTick)
		}
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "pickup denied"})
		return
	}

	a.Inventory[e.Item] += e.Count
	w.removeItemEntity(nowTick, a.ID, id, "GATHER")

	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}

func (w *World) tickPlace(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	pos := v3FromTask(wt.BlockPos)
	if !w.chunks.inBounds(pos) {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "out of bounds"})
		return
	}
	if !w.canBuildAt(a.ID, pos, nowTick) {
		a.WorkTask = nil
		w.bumpRepLaw(a.ID, -1)
		if w.stats != nil {
			w.stats.RecordDenied(nowTick)
		}
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "build denied"})
		return
	}
	if w.chunks.GetBlock(pos) != w.chunks.gen.Air {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "space occupied"})
		return
	}
	if wt.ItemID == "" || a.Inventory[wt.ItemID] < 1 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "missing item"})
		return
	}

	blockName := wt.ItemID
	if def, ok := w.catalogs.Items.Defs[wt.ItemID]; ok && def.PlaceAs != "" {
		blockName = def.PlaceAs
	}
	bid, ok := w.catalogs.Blocks.Index[blockName]
	if !ok {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "item not placeable"})
		return
	}

	a.Inventory[wt.ItemID]--
	w.chunks.SetBlock(pos, bid)
	w.auditSetBlock(nowTick, a.ID, pos, w.chunks.gen.Air, bid, "PLACE")
	w.ensureContainerForPlacedBlock(pos, blockName)
	if blockName == "CONVEYOR" {
		dx, dz := yawToDir(a.Yaw)
		w.ensureConveyor(pos, dx, dz)
	}

	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}

func (w *World) tickOpen(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	c := w.getContainerByID(wt.TargetID)
	if c == nil {
		// Fallback: allow OPEN on bulletin boards ("BULLETIN_BOARD@x,y,z") to read posts.
		if typ, pos, ok := parseContainerID(wt.TargetID); ok && typ == "BULLETIN_BOARD" {
			if ok, code, msg := interactpkg.ValidateBoardOpen(w.blockName(w.chunks.GetBlock(pos)), Manhattan(a.Pos, pos)); !ok {
				a.WorkTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
				return
			}
			bid := boardIDAt(pos)
			b := w.boards[bid]
			if b == nil {
				b = w.ensureBoard(pos)
			}

			boardPosts := make([]interactpkg.BoardPost, 0, len(b.Posts))
			for _, p := range b.Posts {
				boardPosts = append(boardPosts, interactpkg.BoardPost{
					PostID: p.PostID,
					Author: p.Author,
					Title:  p.Title,
					Body:   p.Body,
					Tick:   p.Tick,
				})
			}
			posts := interactpkg.BuildBoardPosts(boardPosts, 20)
			a.AddEvent(protocol.Event{
				"t":           nowTick,
				"type":        "BOARD",
				"board_id":    bid,
				"pos":         pos.ToArray(),
				"total_posts": len(b.Posts),
				"posts":       posts,
			})
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
			return
		}

		// Fallback: allow OPEN on signs ("SIGN@x,y,z") to read text.
		if typ, pos, ok := parseContainerID(wt.TargetID); ok && typ == "SIGN" {
			if ok, code, msg := interactpkg.ValidateSignOpen(w.blockName(w.chunks.GetBlock(pos)), Manhattan(a.Pos, pos)); !ok {
				a.WorkTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
				return
			}
			s := w.signs[pos]
			text := ""
			updatedTick := uint64(0)
			updatedBy := ""
			if s != nil {
				text = s.Text
				updatedTick = s.UpdatedTick
				updatedBy = s.UpdatedBy
			}
			a.AddEvent(protocol.Event{
				"t":            nowTick,
				"type":         "SIGN",
				"sign_id":      signIDAt(pos),
				"pos":          pos.ToArray(),
				"text":         text,
				"updated_tick": updatedTick,
				"updated_by":   updatedBy,
			})
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
			return
		}

		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "container not found"})
		return
	}
	if Manhattan(a.Pos, c.Pos) > 3 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "too far"})
		return
	}

	ev := protocol.Event{
		"t":              nowTick,
		"type":           "CONTAINER",
		"container":      c.ID(),
		"container_type": c.Type,
		"pos":            c.Pos.ToArray(),
		"inventory":      c.InventoryList(),
	}
	// Include owed summary for this agent.
	if c.Owed != nil {
		if owed := c.Owed[a.ID]; owed != nil {
			ev["owed"] = inventorypkg.EncodeItemPairs(owed)
		}
	}
	// Include contract summaries if it's a terminal.
	if c.Type == "CONTRACT_TERMINAL" {
		ev["contracts"] = w.contractSummariesForTerminal(c.Pos)
	}
	a.AddEvent(ev)
	w.onContainerOpenedDuringEvent(a, c, nowTick)

	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}

func (w *World) tickTransfer(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	srcID := wt.SrcContainer
	dstID := wt.DstContainer
	item := wt.ItemID
	n := wt.Count

	if ok, code, msg := interactpkg.ValidateTransferNoop(srcID, dstID); !ok {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
		return
	}

	var srcC *Container
	var dstC *Container
	if srcID != "SELF" {
		srcC = w.getContainerByID(srcID)
		srcDist := 0
		if srcC != nil {
			srcDist = Manhattan(a.Pos, srcC.Pos)
		}
		if ok, code, msg := interactpkg.ValidateContainerDistance(srcC != nil, srcDist, "src"); !ok {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
			return
		}
	}
	if dstID != "SELF" {
		dstC = w.getContainerByID(dstID)
		dstDist := 0
		if dstC != nil {
			dstDist = Manhattan(a.Pos, dstC.Pos)
		}
		if ok, code, msg := interactpkg.ValidateContainerDistance(dstC != nil, dstDist, "dst"); !ok {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
			return
		}
	}

	// Withdraw permission and escrow protection.
	if srcC != nil {
		if !w.canWithdrawFromContainer(a.ID, srcC.Pos) {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "withdraw denied"})
			return
		}
		if srcC.AvailableCount(item) < n {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "insufficient src items"})
			return
		}
	} else {
		if a.Inventory[item] < n {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "insufficient self items"})
			return
		}
	}

	// Execute transfer.
	if srcC != nil {
		srcC.Inventory[item] -= n
		if srcC.Inventory[item] <= 0 {
			delete(srcC.Inventory, item)
		}
	} else {
		a.Inventory[item] -= n
	}
	if dstC != nil {
		if dstC.Inventory == nil {
			dstC.Inventory = map[string]int{}
		}
		dstC.Inventory[item] += n
	} else {
		a.Inventory[item] += n
	}

	// Audit the transfer for dispute resolution.
	ap := a.Pos
	if dstC != nil {
		ap = dstC.Pos
	} else if srcC != nil {
		ap = srcC.Pos
	}
	w.auditEvent(nowTick, a.ID, "TRANSFER", ap, "TRANSFER", map[string]any{
		"src":   srcID,
		"dst":   dstID,
		"item":  item,
		"count": n,
	})

	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}

func (w *World) tickCraft(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	rec := w.catalogs.Recipes.ByID[wt.RecipeID]
	// Station constraint (MVP): must be within 2 blocks of a crafting bench block.
	switch rec.Station {
	case "HAND":
		// no constraint
	case "CRAFTING_BENCH":
		if !w.nearBlock(a.Pos, "CRAFTING_BENCH", 2) {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "need crafting bench nearby"})
			return
		}
	default:
		// Unknown station for CRAFT.
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "unsupported station"})
		return
	}

	wt.WorkTicks++
	if wt.WorkTicks < rec.TimeTicks {
		return
	}
	wt.WorkTicks = 0

	// Check + consume inputs.
	for _, in := range rec.Inputs {
		if a.Inventory[in.Item] < in.Count {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "missing inputs"})
			return
		}
	}
	for _, in := range rec.Inputs {
		a.Inventory[in.Item] -= in.Count
	}
	for _, out := range rec.Outputs {
		a.Inventory[out.Item] += out.Count
	}
	w.funOnRecipe(a, wt.RecipeID, rec.Tier, nowTick)

	wt.Count--
	if wt.Count <= 0 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
	}
}

func (w *World) tickSmelt(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	// MVP: require furnace nearby for any smelt.
	if !w.nearBlock(a.Pos, "FURNACE", 2) {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "need furnace nearby"})
		return
	}

	rec, ok := w.smeltByInput[wt.ItemID]
	if !ok {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "unsupported smelt item"})
		return
	}
	wt.WorkTicks++
	if wt.WorkTicks < rec.TimeTicks {
		return
	}
	wt.WorkTicks = 0

	// Check + consume inputs.
	for _, in := range rec.Inputs {
		if a.Inventory[in.Item] < in.Count {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "missing inputs"})
			return
		}
	}
	for _, in := range rec.Inputs {
		a.Inventory[in.Item] -= in.Count
	}
	for _, out := range rec.Outputs {
		a.Inventory[out.Item] += out.Count
	}
	w.funOnRecipe(a, rec.RecipeID, rec.Tier, nowTick)

	wt.Count--
	if wt.Count <= 0 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
	}
}

func (w *World) tickBuildBlueprint(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	bp := w.catalogs.Blueprints.ByID[wt.BlueprintID]
	anchor := v3FromTask(wt.Anchor)
	rot := blueprint.NormalizeRotation(wt.Rotation)

	// On first tick, validate cost.
	if wt.BuildIndex == 0 && wt.WorkTicks == 0 {
		// Preflight: space + permission check so we don't consume materials and then fail immediately.
		// Also allow resuming: if a target cell already contains the correct block, treat it as satisfied.
		alreadyCorrect := map[string]int{}
		correct := 0
		for _, p := range bp.Blocks {
			off := blueprint.RotateOffset(p.Pos, rot)
			pos := Vec3i{
				X: anchor.X + off[0],
				Y: anchor.Y + off[1],
				Z: anchor.Z + off[2],
			}
			if !w.chunks.inBounds(pos) {
				a.WorkTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "out of bounds"})
				return
			}
			bid, ok := w.catalogs.Blocks.Index[p.Block]
			if !ok {
				a.WorkTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INTERNAL", "message": "unknown block in blueprint"})
				return
			}
			if !w.canBuildAt(a.ID, pos, nowTick) {
				a.WorkTask = nil
				w.bumpRepLaw(a.ID, -1)
				if w.stats != nil {
					w.stats.RecordDenied(nowTick)
				}
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "build denied"})
				return
			}
			cur := w.chunks.GetBlock(pos)
			if cur != w.chunks.gen.Air {
				if cur == bid {
					alreadyCorrect[p.Block]++
					correct++
					continue
				}
				a.WorkTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "space occupied"})
				return
			}
		}

		// Anti-exploit: if the entire blueprint is already present, treat as no-op completion
		// (no cost, no structure registration, no fun/stats).
		if blueprint.FullySatisfied(correct, len(bp.Blocks)) {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
			return
		}

		// Charge only for the remaining required materials (best-effort: subtract already-correct blocks by id).
		baseCost := make([]blueprint.ItemCount, 0, len(bp.Cost))
		for _, c := range bp.Cost {
			if strings.TrimSpace(c.Item) == "" || c.Count <= 0 {
				continue
			}
			baseCost = append(baseCost, blueprint.ItemCount{Item: c.Item, Count: c.Count})
		}
		remaining := blueprint.RemainingCost(baseCost, alreadyCorrect)
		needCost := make([]catalogs.ItemCount, 0, len(remaining))
		for _, c := range remaining {
			needCost = append(needCost, catalogs.ItemCount{Item: c.Item, Count: c.Count})
		}

		for _, c := range needCost {
			if a.Inventory[c.Item] < c.Count {
				// Try auto-pull from nearby storage (same land, within range) if possible.
				if ok, msg := w.blueprintEnsureMaterials(a, anchor, needCost, nowTick); !ok {
					a.WorkTask = nil
					if msg == "" {
						msg = "missing materials"
					}
					a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": msg})
					return
				}
				break
			}
		}
		for _, c := range needCost {
			a.Inventory[c.Item] -= c.Count
		}
	}

	// Place up to N blocks per tick (default 2).
	placed := 0
	limit := limitspkg.ClampBlocksPerTick(w.cfg.BlueprintBlocksPerTick)
	for placed < limit && wt.BuildIndex < len(bp.Blocks) {
		p := bp.Blocks[wt.BuildIndex]
		off := blueprint.RotateOffset(p.Pos, rot)
		pos := Vec3i{
			X: anchor.X + off[0],
			Y: anchor.Y + off[1],
			Z: anchor.Z + off[2],
		}
		if !w.chunks.inBounds(pos) {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "out of bounds"})
			return
		}
		bid, ok := w.catalogs.Blocks.Index[p.Block]
		if !ok {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INTERNAL", "message": "unknown block in blueprint"})
			return
		}
		if !w.canBuildAt(a.ID, pos, nowTick) {
			a.WorkTask = nil
			w.bumpRepLaw(a.ID, -1)
			if w.stats != nil {
				w.stats.RecordDenied(nowTick)
			}
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "build denied"})
			return
		}
		cur := w.chunks.GetBlock(pos)
		if cur != w.chunks.gen.Air {
			if cur == bid {
				wt.BuildIndex++
				continue
			}
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "space occupied"})
			return
		}
		w.chunks.SetBlock(pos, bid)
		w.auditSetBlock(nowTick, a.ID, pos, w.chunks.gen.Air, bid, "BUILD_BLUEPRINT")
		w.ensureContainerForPlacedBlock(pos, p.Block)

		wt.BuildIndex++
		placed++
	}

	if wt.BuildIndex >= len(bp.Blocks) {
		if w.stats != nil {
			w.stats.RecordBlueprintComplete(nowTick)
		}
		w.registerStructure(nowTick, a.ID, wt.BlueprintID, anchor, rot)
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
}

func handleTaskClaimLand(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if !w.cfg.AllowClaims {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_PERMISSION", "claims disabled in this world"))
		return
	}
	r := tr.Radius
	if r <= 0 {
		r = 32
	}
	if r > 128 {
		r = 128
	}
	if tr.Anchor[1] != 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "2D world requires y==0"))
		return
	}
	anchor := Vec3i{X: tr.Anchor[0], Y: tr.Anchor[1], Z: tr.Anchor[2]}
	if !w.chunks.inBounds(anchor) {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "out of bounds"))
		return
	}
	// Must be allowed to build at anchor (unclaimed or owned land with build permission).
	if !w.canBuildAt(a.ID, anchor, nowTick) {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_PERMISSION", "cannot claim here"))
		return
	}
	// Must have resources: 1 battery + 1 crystal shard (MVP).
	if a.Inventory["BATTERY"] < 1 || a.Inventory["CRYSTAL_SHARD"] < 1 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_RESOURCE", "need BATTERY + CRYSTAL_SHARD"))
		return
	}
	// Must not overlap existing claims.
	for _, c := range w.claims {
		// Conservative overlap check: if anchors are close enough, treat as overlap.
		if mathx.AbsInt(anchor.X-c.Anchor.X) <= r+c.Radius && mathx.AbsInt(anchor.Z-c.Anchor.Z) <= r+c.Radius {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "claim overlaps existing land"))
			return
		}
	}
	// Place Claim Totem block at anchor.
	if w.chunks.GetBlock(anchor) != w.chunks.gen.Air {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BLOCKED", "anchor occupied"))
		return
	}
	totemID, ok := w.catalogs.Blocks.Index["CLAIM_TOTEM"]
	if !ok {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INTERNAL", "missing CLAIM_TOTEM block"))
		return
	}

	// Consume cost.
	a.Inventory["BATTERY"]--
	a.Inventory["CRYSTAL_SHARD"]--

	w.chunks.SetBlock(anchor, totemID)
	w.auditSetBlock(nowTick, a.ID, anchor, w.chunks.gen.Air, totemID, "CLAIM_LAND")

	landID := w.newLandID(a.ID)
	claimType := claimspkg.DefaultClaimTypeForWorld(w.cfg.WorldType)
	baseFlags := claimspkg.DefaultFlags(claimType)
	due := uint64(0)
	if w.cfg.DayTicks > 0 {
		due = nowTick + uint64(w.cfg.DayTicks)
	}
	w.claims[landID] = &LandClaim{
		LandID:    landID,
		Owner:     a.ID,
		ClaimType: claimType,
		Anchor:    anchor,
		Radius:    r,
		Flags: ClaimFlags{
			AllowBuild:  baseFlags.AllowBuild,
			AllowBreak:  baseFlags.AllowBreak,
			AllowDamage: baseFlags.AllowDamage,
			AllowTrade:  baseFlags.AllowTrade,
		},
		MaintenanceDueTick: due,
		MaintenanceStage:   0,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "land_id": landID})
}

func handleTaskBuildBlueprint(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if !w.cfg.AllowBuild {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_PERMISSION", "blueprint build disabled in this world"))
		return
	}
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.BlueprintID == "" {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing blueprint_id"))
		return
	}
	if tr.Anchor[1] != 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "2D world requires y==0"))
		return
	}
	if _, ok := w.catalogs.Blueprints.ByID[tr.BlueprintID]; !ok {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "unknown blueprint"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindBuildBlueprint,
		BlueprintID: tr.BlueprintID,
		Anchor:      tasks.Vec3i{X: tr.Anchor[0], Y: 0, Z: tr.Anchor[2]},
		Rotation:    blueprint.NormalizeRotation(tr.Rotation),
		BuildIndex:  0,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}
