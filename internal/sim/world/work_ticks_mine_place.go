package world

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	featurework "voxelcraft.ai/internal/sim/world/feature/work"
)

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

	family := featurework.MineToolFamilyForBlock(blockName)
	tier := featurework.BestToolTier(a.Inventory, family)
	mineWorkNeeded, mineCost := featurework.MineParamsForTier(tier)

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
