package world

import (
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
	conveyorID, ok := w.catalogs.Blocks.Index["CONVEYOR"]
	if !ok {
		return
	}
	conveyruntimepkg.Run(
		nowTick,
		conveyorID,
		w.conveyors,
		w.items,
		w.itemsAt,
		w.containers,
		conveyruntimepkg.Ops{
			ConveyorEnabled: w.conveyorEnabled,
			BlockAt:         w.chunks.GetBlock,
			BlockSolid:      w.blockSolid,
			BlockName:       w.blockName,
			AuditEvent:      w.auditEvent,
			RemoveItem:      w.removeItemEntity,
			MoveItem:        w.moveItemEntity,
			SpawnItem:       w.spawnItemEntity,
		},
	)
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
	return conveyruntimepkg.SensorOn(
		pos,
		func(p Vec3i) string { return w.blockName(w.chunks.GetBlock(p)) },
		func(p Vec3i) []string { return w.itemsAt[p] },
		func(id string) (conveyruntimepkg.ItemEntry, bool) {
			e := w.items[id]
			if e == nil {
				return conveyruntimepkg.ItemEntry{}, false
			}
			return conveyruntimepkg.ItemEntry{
				ID:    id,
				Item:  e.Item,
				Count: e.Count,
			}, true
		},
		func(p Vec3i) bool {
			c := w.containers[p]
			if c == nil || len(c.Inventory) == 0 {
				return false
			}
			for item := range c.Inventory {
				if c.AvailableCount(item) > 0 {
					return true
				}
			}
			return false
		},
	)
}
