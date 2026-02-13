package runtime

import (
	"sort"

	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

// Ops is the world-facing callback set used by conveyor runtime.
// It intentionally keeps mutations in the caller (world facade) while
// this package handles deterministic conveyor decision flow.
type Ops struct {
	ConveyorEnabled func(pos modelpkg.Vec3i) bool
	BlockAt         func(pos modelpkg.Vec3i) uint16
	BlockSolid      func(block uint16) bool
	BlockName       func(block uint16) string

	AuditEvent func(nowTick uint64, actor, action string, pos modelpkg.Vec3i, reason string, details map[string]any)
	RemoveItem func(nowTick uint64, actor, itemID string, reason string)
	MoveItem   func(nowTick uint64, actor, itemID string, to modelpkg.Vec3i, reason string)
	SpawnItem  func(nowTick uint64, actor string, pos modelpkg.Vec3i, item string, count int, reason string) string
}

func sortedLiveItemIDs(items map[string]*modelpkg.ItemEntity) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for id, e := range items {
		if id == "" || e == nil || e.Item == "" || e.Count <= 0 {
			continue
		}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func sortedConveyorPositions(conveyors map[modelpkg.Vec3i]modelpkg.ConveyorMeta, blockAt func(modelpkg.Vec3i) uint16, conveyorID uint16) []modelpkg.Vec3i {
	if len(conveyors) == 0 {
		return nil
	}
	out := make([]modelpkg.Vec3i, 0, len(conveyors))
	for p := range conveyors {
		if blockAt != nil && blockAt(p) != conveyorID {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].X != out[j].X {
			return out[i].X < out[j].X
		}
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].Z < out[j].Z
	})
	return out
}

// Run executes one deterministic conveyor tick over item entities and container pull.
func Run(
	nowTick uint64,
	conveyorID uint16,
	conveyors map[modelpkg.Vec3i]modelpkg.ConveyorMeta,
	items map[string]*modelpkg.ItemEntity,
	itemsAt map[modelpkg.Vec3i][]string,
	containers map[modelpkg.Vec3i]*modelpkg.Container,
	ops Ops,
) {
	if len(conveyors) == 0 {
		return
	}

	// Pass 1: move existing item entities along conveyors.
	if len(items) > 0 {
		for _, id := range sortedLiveItemIDs(items) {
			e := items[id]
			if e == nil || e.Item == "" || e.Count <= 0 {
				continue
			}
			if ops.BlockAt != nil && ops.BlockAt(e.Pos) != conveyorID {
				continue
			}
			meta, ok := conveyors[e.Pos]
			if !ok || (meta.DX == 0 && meta.DZ == 0) {
				continue
			}
			if ops.ConveyorEnabled != nil && !ops.ConveyorEnabled(e.Pos) {
				continue
			}

			to := modelpkg.Vec3i{X: e.Pos.X + int(meta.DX), Y: e.Pos.Y, Z: e.Pos.Z + int(meta.DZ)}

			// Insert into containers when present.
			if c := containers[to]; c != nil {
				if c.Inventory == nil {
					c.Inventory = map[string]int{}
				}
				c.Inventory[e.Item] += e.Count
				if ops.AuditEvent != nil {
					ops.AuditEvent(nowTick, "WORLD", "CONVEYOR_INSERT", to, "CONVEYOR", map[string]any{
						"entity_id":    e.EntityID,
						"from":         e.Pos.ToArray(),
						"container_id": c.ID(),
						"item":         e.Item,
						"count":        e.Count,
					})
				}
				if ops.RemoveItem != nil {
					ops.RemoveItem(nowTick, "WORLD", id, "CONVEYOR_INSERT")
				}
				continue
			}

			// Only move onto non-solid blocks, except conveyors themselves (which are solid for agents).
			var b uint16
			if ops.BlockAt != nil {
				b = ops.BlockAt(to)
			}
			if ops.BlockSolid != nil && ops.BlockSolid(b) {
				if ops.BlockName == nil || ops.BlockName(b) != "CONVEYOR" {
					continue
				}
			}

			if ops.MoveItem != nil {
				ops.MoveItem(nowTick, "WORLD", id, to, "CONVEYOR_MOVE")
			}
		}
	}

	// Pass 2: pull from container behind the conveyor onto the belt when belt cell is empty.
	for _, p := range sortedConveyorPositions(conveyors, ops.BlockAt, conveyorID) {
		meta := conveyors[p]
		if meta.DX == 0 && meta.DZ == 0 {
			continue
		}
		if ops.ConveyorEnabled != nil && !ops.ConveyorEnabled(p) {
			continue
		}

		// Skip if there is any live item entity already on this belt cell.
		if ids := itemsAt[p]; len(ids) > 0 {
			hasLive := false
			for _, id := range ids {
				e := items[id]
				if e != nil && e.Item != "" && e.Count > 0 {
					hasLive = true
					break
				}
			}
			if hasLive {
				continue
			}
		}

		back := modelpkg.Vec3i{X: p.X - int(meta.DX), Y: p.Y, Z: p.Z - int(meta.DZ)}
		c := containers[back]
		if c == nil {
			continue
		}
		item := PickAvailableItem(c.Inventory, c.AvailableCount)
		if item == "" {
			continue
		}

		// Pull exactly 1 unit per tick per conveyor.
		c.Inventory[item]--
		if c.Inventory[item] <= 0 {
			delete(c.Inventory, item)
		}
		if ops.SpawnItem != nil {
			_ = ops.SpawnItem(nowTick, "WORLD", p, item, 1, "CONVEYOR_PULL")
		}
		if ops.AuditEvent != nil {
			ops.AuditEvent(nowTick, "WORLD", "CONVEYOR_PULL", p, "CONVEYOR", map[string]any{
				"from":  back.ToArray(),
				"item":  item,
				"count": 1,
			})
		}
	}
}
