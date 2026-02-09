package world

import "sort"

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
		itemIDs := make([]string, 0, len(w.items))
		for id, e := range w.items {
			if e == nil || e.Item == "" || e.Count <= 0 {
				continue
			}
			itemIDs = append(itemIDs, id)
		}
		sort.Strings(itemIDs)

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
		item := pickAvailableContainerItem(c)
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

func pickAvailableContainerItem(c *Container) string {
	if c == nil || len(c.Inventory) == 0 {
		return ""
	}
	keys := make([]string, 0, len(c.Inventory))
	for item, n := range c.Inventory {
		if item == "" || n <= 0 {
			continue
		}
		if c.availableCount(item) <= 0 {
			continue
		}
		keys = append(keys, item)
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	return keys[0]
}

func (w *World) conveyorEnabled(pos Vec3i) bool {
	dirs := []Vec3i{
		{X: 1, Y: 0, Z: 0},
		{X: -1, Y: 0, Z: 0},
		{X: 0, Y: 0, Z: 1},
		{X: 0, Y: 0, Z: -1},
	}

	// Rule 1: adjacent control blocks act as direct enable signals.
	// If any adjacent switch/sensor exists, require at least one to be ON.
	foundControl := false
	for _, d := range dirs {
		p := Vec3i{X: pos.X + d.X, Y: pos.Y + d.Y, Z: pos.Z + d.Z}
		switch w.blockName(w.chunks.GetBlock(p)) {
		case "SWITCH":
			foundControl = true
			if w.switches[p] {
				return true
			}
		case "SENSOR":
			foundControl = true
			if w.sensorOn(p) {
				return true
			}
		}
	}
	if foundControl {
		return false
	}

	// Rule 2: adjacent wires form a simple network; if any adjacent wire exists, require the
	// wire network to connect to an ON switch or active sensor (within a capped BFS budget).
	wireStarts := make([]Vec3i, 0, 4)
	for _, d := range dirs {
		p := Vec3i{X: pos.X + d.X, Y: pos.Y + d.Y, Z: pos.Z + d.Z}
		if w.blockName(w.chunks.GetBlock(p)) == "WIRE" {
			wireStarts = append(wireStarts, p)
		}
	}
	if len(wireStarts) > 0 {
		return w.wirePoweredBySwitch(wireStarts, 1024)
	}

	// No control blocks nearby -> enabled by default.
	return true
}

func (w *World) wirePoweredBySwitch(starts []Vec3i, maxNodes int) bool {
	if len(starts) == 0 || maxNodes <= 0 {
		return false
	}
	dirs := []Vec3i{
		{X: 1, Y: 0, Z: 0},
		{X: -1, Y: 0, Z: 0},
		{X: 0, Y: 0, Z: 1},
		{X: 0, Y: 0, Z: -1},
	}

	visited := map[Vec3i]bool{}
	q := make([]Vec3i, 0, len(starts))
	for _, p := range starts {
		if w.blockName(w.chunks.GetBlock(p)) != "WIRE" {
			continue
		}
		if visited[p] {
			continue
		}
		visited[p] = true
		q = append(q, p)
	}

	for len(q) > 0 && len(visited) <= maxNodes {
		p := q[0]
		q = q[1:]

		// Check adjacent switches/sensors.
		for _, d := range dirs {
			sp := Vec3i{X: p.X + d.X, Y: p.Y + d.Y, Z: p.Z + d.Z}
			switch w.blockName(w.chunks.GetBlock(sp)) {
			case "SWITCH":
				if w.switches[sp] {
					return true
				}
			case "SENSOR":
				if w.sensorOn(sp) {
					return true
				}
			}
		}

		// Expand to neighboring wires.
		for _, d := range dirs {
			np := Vec3i{X: p.X + d.X, Y: p.Y + d.Y, Z: p.Z + d.Z}
			if visited[np] {
				continue
			}
			if w.blockName(w.chunks.GetBlock(np)) != "WIRE" {
				continue
			}
			visited[np] = true
			q = append(q, np)
			if len(visited) > maxNodes {
				break
			}
		}
	}
	return false
}
