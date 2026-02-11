package world

import "voxelcraft.ai/internal/persistence/snapshot"

func (w *World) importSnapshotContainers(s snapshot.SnapshotV1) {
	w.containers = map[Vec3i]*Container{}
	for _, c := range s.Containers {
		pos := Vec3i{X: c.Pos[0], Y: c.Pos[1], Z: c.Pos[2]}
		cc := &Container{
			Type:      c.Type,
			Pos:       pos,
			Inventory: map[string]int{},
		}
		for item, n := range c.Inventory {
			if n > 0 {
				cc.Inventory[item] = n
			}
		}
		if len(c.Reserved) > 0 {
			cc.Reserved = map[string]int{}
			for item, n := range c.Reserved {
				if n > 0 {
					cc.Reserved[item] = n
				}
			}
		}
		if len(c.Owed) > 0 {
			cc.Owed = map[string]map[string]int{}
			for aid, m := range c.Owed {
				if aid == "" || len(m) == 0 {
					continue
				}
				m2 := map[string]int{}
				for item, n := range m {
					if n > 0 {
						m2[item] = n
					}
				}
				if len(m2) > 0 {
					cc.Owed[aid] = m2
				}
			}
		}
		w.containers[pos] = cc
	}
}

func (w *World) importSnapshotItems(s snapshot.SnapshotV1) (maxItem uint64) {
	w.items = map[string]*ItemEntity{}
	w.itemsAt = map[Vec3i][]string{}
	for _, it := range s.Items {
		if it.EntityID == "" || it.Item == "" || it.Count <= 0 {
			continue
		}
		if it.ExpiresTick != 0 && s.Header.Tick >= it.ExpiresTick {
			continue
		}
		pos := Vec3i{X: it.Pos[0], Y: it.Pos[1], Z: it.Pos[2]}
		e := &ItemEntity{
			EntityID:    it.EntityID,
			Pos:         pos,
			Item:        it.Item,
			Count:       it.Count,
			CreatedTick: it.CreatedTick,
			ExpiresTick: it.ExpiresTick,
		}
		w.items[e.EntityID] = e
		w.itemsAt[pos] = append(w.itemsAt[pos], e.EntityID)
		if n, ok := parseUintAfterPrefix("IT", e.EntityID); ok && n > maxItem {
			maxItem = n
		}
	}
	return maxItem
}

func (w *World) importSnapshotSigns(s snapshot.SnapshotV1) {
	w.signs = map[Vec3i]*Sign{}
	for _, ss := range s.Signs {
		pos := Vec3i{X: ss.Pos[0], Y: ss.Pos[1], Z: ss.Pos[2]}
		if w.blockName(w.chunks.GetBlock(pos)) != "SIGN" {
			continue
		}
		w.signs[pos] = &Sign{
			Pos:         pos,
			Text:        ss.Text,
			UpdatedTick: ss.UpdatedTick,
			UpdatedBy:   ss.UpdatedBy,
		}
	}
}

func (w *World) importSnapshotConveyors(s snapshot.SnapshotV1) {
	w.conveyors = map[Vec3i]ConveyorMeta{}
	for _, cv := range s.Conveyors {
		pos := Vec3i{X: cv.Pos[0], Y: cv.Pos[1], Z: cv.Pos[2]}
		if w.blockName(w.chunks.GetBlock(pos)) != "CONVEYOR" {
			continue
		}
		w.conveyors[pos] = ConveyorMeta{DX: int8(cv.DX), DZ: int8(cv.DZ)}
	}
}

func (w *World) importSnapshotSwitches(s snapshot.SnapshotV1) {
	w.switches = map[Vec3i]bool{}
	for _, sv := range s.Switches {
		pos := Vec3i{X: sv.Pos[0], Y: sv.Pos[1], Z: sv.Pos[2]}
		if w.blockName(w.chunks.GetBlock(pos)) != "SWITCH" {
			continue
		}
		w.switches[pos] = sv.On
	}
}
