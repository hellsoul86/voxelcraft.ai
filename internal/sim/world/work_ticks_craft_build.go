package world

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/tasks"
)

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
	rot := normalizeRotation(wt.Rotation)

	// On first tick, validate cost.
	if wt.BuildIndex == 0 && wt.WorkTicks == 0 {
		// Preflight: space + permission check so we don't consume materials and then fail immediately.
		// Also allow resuming: if a target cell already contains the correct block, treat it as satisfied.
		alreadyCorrect := map[string]int{}
		correct := 0
		for _, p := range bp.Blocks {
			off := rotateOffset(p.Pos, rot)
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
		if correct == len(bp.Blocks) {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
			return
		}

		// Charge only for the remaining required materials (best-effort: subtract already-correct blocks by id).
		needCost := make([]catalogs.ItemCount, 0, len(bp.Cost))
		for _, c := range bp.Cost {
			if strings.TrimSpace(c.Item) == "" || c.Count <= 0 {
				continue
			}
			n := c.Count
			if k := alreadyCorrect[c.Item]; k > 0 {
				if k >= n {
					n = 0
				} else {
					n -= k
				}
			}
			if n > 0 {
				needCost = append(needCost, catalogs.ItemCount{Item: c.Item, Count: n})
			}
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
	limit := w.cfg.BlueprintBlocksPerTick
	if limit <= 0 {
		limit = 2
	}
	for placed < limit && wt.BuildIndex < len(bp.Blocks) {
		p := bp.Blocks[wt.BuildIndex]
		off := rotateOffset(p.Pos, rot)
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
