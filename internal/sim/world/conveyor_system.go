package world

import (
	"sort"

	conveyruntimepkg "voxelcraft.ai/internal/sim/world/feature/conveyor/runtime"
	"voxelcraft.ai/internal/sim/world/logic/conveyorpower"
)

// systemConveyors moves dropped item entities along conveyor blocks.
//
// This is a deliberately simple "logistics automation" primitive:
// - One move per tick per item entity.
// - Items can move onto conveyors and other non-solid blocks.
// - If the next block is a container, items are inserted into its inventory.
func (w *World) systemConveyors(nowTick uint64) {
	if len(w.conveyors) == 0 {
		return
	}
	conveyorID, ok := w.catalogs.Blocks.Index["CONVEYOR"]
	if !ok {
		return
	}

	// Pass 1: move existing item entities along conveyors.
	if len(w.items) > 0 {
		items := make(map[string]conveyruntimepkg.ItemEntry, len(w.items))
		for id, e := range w.items {
			if e == nil || e.Item == "" || e.Count <= 0 {
				continue
			}
			items[id] = conveyruntimepkg.ItemEntry{
				ID:    id,
				Item:  e.Item,
				Count: e.Count,
			}
		}
		itemIDs := conveyruntimepkg.SortedLiveItemIDs(items)

		for _, id := range itemIDs {
			e := w.items[id]
			if e == nil || e.Item == "" || e.Count <= 0 {
				continue
			}
			if w.chunks.GetBlock(e.Pos) != conveyorID {
				continue
			}
			meta, ok := w.conveyors[e.Pos]
			if !ok || (meta.DX == 0 && meta.DZ == 0) {
				continue
			}
			if !w.conveyorEnabled(e.Pos) {
				continue
			}

			to := Vec3i{X: e.Pos.X + int(meta.DX), Y: e.Pos.Y, Z: e.Pos.Z + int(meta.DZ)}

			// Insert into containers when present.
			if c := w.containers[to]; c != nil {
				if c.Inventory == nil {
					c.Inventory = map[string]int{}
				}
				c.Inventory[e.Item] += e.Count
				w.auditEvent(nowTick, "WORLD", "CONVEYOR_INSERT", to, "CONVEYOR", map[string]any{
					"entity_id":    e.EntityID,
					"from":         e.Pos.ToArray(),
					"container_id": c.ID(),
					"item":         e.Item,
					"count":        e.Count,
				})
				w.removeItemEntity(nowTick, "WORLD", id, "CONVEYOR_INSERT")
				continue
			}

			// Only move onto non-solid blocks, except conveyors themselves (which are solid for agents).
			b := w.chunks.GetBlock(to)
			if w.blockSolid(b) && w.blockName(b) != "CONVEYOR" {
				continue
			}

			w.moveItemEntity(nowTick, "WORLD", id, to, "CONVEYOR_MOVE")
		}
	}

	// Pass 2: pull from a container behind the conveyor onto the belt when the belt cell is empty.
	convPos := make([]Vec3i, 0, len(w.conveyors))
	for p := range w.conveyors {
		if w.chunks.GetBlock(p) != conveyorID {
			continue
		}
		convPos = append(convPos, p)
	}
	sort.Slice(convPos, func(i, j int) bool {
		if convPos[i].X != convPos[j].X {
			return convPos[i].X < convPos[j].X
		}
		if convPos[i].Y != convPos[j].Y {
			return convPos[i].Y < convPos[j].Y
		}
		return convPos[i].Z < convPos[j].Z
	})
	for _, p := range convPos {
		meta := w.conveyors[p]
		if meta.DX == 0 && meta.DZ == 0 {
			continue
		}
		if !w.conveyorEnabled(p) {
			continue
		}
		// Skip if there is any active item entity already on this belt cell.
		if ids := w.itemsAt[p]; len(ids) > 0 {
			has := false
			for _, id := range ids {
				e := w.items[id]
				if e != nil && e.Item != "" && e.Count > 0 {
					has = true
					break
				}
			}
			if has {
				continue
			}
		}

		back := Vec3i{X: p.X - int(meta.DX), Y: p.Y, Z: p.Z - int(meta.DZ)}
		c := w.containers[back]
		if c == nil {
			continue
		}
		item := conveyruntimepkg.PickAvailableItem(c.Inventory, c.availableCount)
		if item == "" {
			continue
		}
		// Pull exactly 1 unit per tick per conveyor.
		c.Inventory[item]--
		if c.Inventory[item] <= 0 {
			delete(c.Inventory, item)
		}
		_ = w.spawnItemEntity(nowTick, "WORLD", p, item, 1, "CONVEYOR_PULL")
		w.auditEvent(nowTick, "WORLD", "CONVEYOR_PULL", p, "CONVEYOR", map[string]any{
			"from":  back.ToArray(),
			"item":  item,
			"count": 1,
		})
	}
}

type conveyorEnvAdapter struct {
	w *World
}

func (a conveyorEnvAdapter) BlockName(p conveyorpower.Pos) string {
	return a.w.blockName(a.w.chunks.GetBlock(Vec3i{X: p.X, Y: p.Y, Z: p.Z}))
}

func (a conveyorEnvAdapter) SwitchOn(p conveyorpower.Pos) bool {
	return a.w.switches[Vec3i{X: p.X, Y: p.Y, Z: p.Z}]
}

func (a conveyorEnvAdapter) SensorOn(p conveyorpower.Pos) bool {
	return a.w.sensorOn(Vec3i{X: p.X, Y: p.Y, Z: p.Z})
}

func (w *World) conveyorEnabled(pos Vec3i) bool {
	return conveyorpower.Enabled(
		conveyorEnvAdapter{w: w},
		conveyorpower.Pos{X: pos.X, Y: pos.Y, Z: pos.Z},
		1024,
	)
}

// sensorOn reports whether the sensor block at pos currently outputs an "ON" signal.
//
// MVP behavior (no configuration UI yet):
// - ON if there is any non-empty dropped item entity on the sensor block or adjacent to it.
// - ON if there is any adjacent container with at least 1 available item (inventory minus reserved).
func (w *World) sensorOn(pos Vec3i) bool {
	if w == nil {
		return false
	}
	if w.blockName(w.chunks.GetBlock(pos)) != "SENSOR" {
		return false
	}

	hasLiveItemAt := func(p Vec3i) bool {
		return conveyruntimepkg.HasLiveItem(w.itemsAt[p], func(id string) (conveyruntimepkg.ItemEntry, bool) {
			e := w.items[id]
			if e == nil {
				return conveyruntimepkg.ItemEntry{}, false
			}
			return conveyruntimepkg.ItemEntry{
				ID:    id,
				Item:  e.Item,
				Count: e.Count,
			}, true
		})
	}

	for _, d := range conveyruntimepkg.SensorNeighborOffsets() {
		p := Vec3i{X: pos.X + d.X, Y: pos.Y + d.Y, Z: pos.Z + d.Z}
		if hasLiveItemAt(p) {
			return true
		}
		if c := w.containers[p]; c != nil && len(c.Inventory) > 0 {
			for item := range c.Inventory {
				if c.availableCount(item) > 0 {
					return true
				}
			}
		}
	}

	return false
}
