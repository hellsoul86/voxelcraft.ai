package world

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/tasks"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	interactpkg "voxelcraft.ai/internal/sim/world/feature/work/interact"
	limitspkg "voxelcraft.ai/internal/sim/world/feature/work/limits"
	miningpkg "voxelcraft.ai/internal/sim/world/feature/work/mining"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
)

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
		if srcC.availableCount(item) < n {
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
				w.addFun(a, nowTick, "CREATION", "builder_expo", w.funDecay(a, "creation:builder_expo", 8, nowTick))
				a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "EXPO_BUILD", "blueprint_id": wt.BlueprintID})
			case "BLUEPRINT_FAIR":
				w.addFun(a, nowTick, "INFLUENCE", "blueprint_fair", w.funDecay(a, "influence:blueprint_fair", 6, nowTick))
				a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "FAIR_BUILD", "blueprint_id": wt.BlueprintID})
			}
		}
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
	}
}
