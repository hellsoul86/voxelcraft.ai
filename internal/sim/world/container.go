package world

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/world/logic/ids"
)

type Container struct {
	Type string
	Pos  Vec3i

	Inventory map[string]int
	Reserved  map[string]int            // escrow-reserved (reward/deposit)
	Owed      map[string]map[string]int // agent_id -> item -> count
}

func (c *Container) ID() string { return containerID(c.Type, c.Pos) }

func (c *Container) InventoryList() []protocol.ItemStack {
	out := make([]protocol.ItemStack, 0, len(c.Inventory))
	for item, n := range c.Inventory {
		if n <= 0 {
			continue
		}
		out = append(out, protocol.ItemStack{Item: item, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Item < out[j].Item })
	return out
}

func (c *Container) reservedCount(item string) int {
	if c.Reserved == nil {
		return 0
	}
	return c.Reserved[item]
}

func (c *Container) availableCount(item string) int {
	return c.Inventory[item] - c.reservedCount(item)
}

func (c *Container) reserve(item string, n int) {
	if n <= 0 {
		return
	}
	if c.Reserved == nil {
		c.Reserved = map[string]int{}
	}
	c.Reserved[item] += n
}

func (c *Container) unreserve(item string, n int) {
	if n <= 0 || c.Reserved == nil {
		return
	}
	c.Reserved[item] -= n
	if c.Reserved[item] <= 0 {
		delete(c.Reserved, item)
	}
}

func (c *Container) addOwed(agentID, item string, n int) {
	if n <= 0 {
		return
	}
	if c.Owed == nil {
		c.Owed = map[string]map[string]int{}
	}
	m := c.Owed[agentID]
	if m == nil {
		m = map[string]int{}
		c.Owed[agentID] = m
	}
	m[item] += n
}

func (c *Container) claimOwed(agentID string) map[string]int {
	if c.Owed == nil {
		return nil
	}
	m := c.Owed[agentID]
	if m == nil {
		return nil
	}
	delete(c.Owed, agentID)
	return m
}

func containerID(typ string, pos Vec3i) string {
	return ids.ContainerID(typ, pos.X, pos.Y, pos.Z)
}

func parseContainerID(id string) (typ string, pos Vec3i, ok bool) {
	typ, x, y, z, ok := ids.ParseContainerID(id)
	if !ok {
		return "", Vec3i{}, false
	}
	return typ, Vec3i{X: x, Y: y, Z: z}, true
}

func (w *World) ensureContainerForPlacedBlock(pos Vec3i, blockName string) {
	switch blockName {
	case "CHEST", "FURNACE", "CONTRACT_TERMINAL":
		w.ensureContainer(pos, blockName)
	case "BULLETIN_BOARD":
		w.ensureBoard(pos)
	case "SIGN":
		w.ensureSign(pos)
	case "CONVEYOR":
		// Blueprint placements don't have a notion of placement yaw yet, so default to +X.
		w.ensureConveyor(pos, 1, 0)
	case "SWITCH":
		w.ensureSwitch(pos, false)
	}
}

func (w *World) ensureContainer(pos Vec3i, typ string) *Container {
	c := w.containers[pos]
	if c != nil {
		// If the type changed (shouldn't happen), overwrite.
		c.Type = typ
		return c
	}
	c = &Container{
		Type:      typ,
		Pos:       pos,
		Inventory: map[string]int{},
	}
	w.containers[pos] = c
	return c
}

func (w *World) removeContainer(pos Vec3i) *Container {
	c := w.containers[pos]
	if c == nil {
		return nil
	}
	delete(w.containers, pos)
	return c
}

func (w *World) getContainerByID(id string) *Container {
	typ, pos, ok := parseContainerID(id)
	if !ok {
		return nil
	}
	c := w.containers[pos]
	if c == nil {
		return nil
	}
	if c.Type != typ {
		return nil
	}
	return c
}

func (w *World) canWithdrawFromContainer(agentID string, pos Vec3i) bool {
	land := w.landAt(pos)
	if land == nil {
		return true
	}
	return w.isLandMember(agentID, land)
}

// --- Block runtime meta: SIGN / CONVEYOR / SWITCH ---

func signIDAt(pos Vec3i) string { return ids.SignIDAt(pos.X, pos.Y, pos.Z) }

func (w *World) ensureSign(pos Vec3i) *Sign {
	s := w.signs[pos]
	if s != nil {
		s.Pos = pos
		return s
	}
	s = &Sign{Pos: pos}
	w.signs[pos] = s
	return s
}

func (w *World) removeSign(nowTick uint64, actor string, pos Vec3i, reason string) {
	s := w.signs[pos]
	if s == nil {
		return
	}
	delete(w.signs, pos)
	// Record the removal as a separate audit event (the SET_BLOCK audit already exists too).
	w.auditEvent(nowTick, actor, "SIGN_REMOVE", pos, reason, map[string]any{
		"sign_id": signIDAt(pos),
	})
}

func (w *World) sortedSignPositionsNear(pos Vec3i, dist int) []Vec3i {
	out := make([]Vec3i, 0, 8)
	for p, s := range w.signs {
		if s == nil {
			continue
		}
		if Manhattan(p, pos) <= dist {
			out = append(out, p)
		}
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

func conveyorIDAt(pos Vec3i) string { return ids.ConveyorIDAt(pos.X, pos.Y, pos.Z) }

func (w *World) ensureConveyor(pos Vec3i, dx, dz int) {
	if dx > 1 {
		dx = 1
	} else if dx < -1 {
		dx = -1
	}
	if dz > 1 {
		dz = 1
	} else if dz < -1 {
		dz = -1
	}
	// Enforce cardinal direction (deterministic tie-break).
	if dx != 0 && dz != 0 {
		dz = 0
	}
	w.conveyors[pos] = ConveyorMeta{DX: int8(dx), DZ: int8(dz)}
}

func (w *World) removeConveyor(nowTick uint64, actor string, pos Vec3i, reason string) {
	if _, ok := w.conveyors[pos]; !ok {
		return
	}
	delete(w.conveyors, pos)
	w.auditEvent(nowTick, actor, "CONVEYOR_REMOVE", pos, reason, map[string]any{
		"conveyor_id": conveyorIDAt(pos),
	})
}

func (w *World) sortedConveyorPositionsNear(pos Vec3i, dist int) []Vec3i {
	out := make([]Vec3i, 0, 8)
	for p := range w.conveyors {
		if Manhattan(p, pos) > dist {
			continue
		}
		// Guard against stale meta (e.g. if blocks were edited without calling removeConveyor).
		if w.blockName(w.chunks.GetBlock(p)) != "CONVEYOR" {
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

func conveyorDirTag(m ConveyorMeta) string {
	switch {
	case m.DX == 1 && m.DZ == 0:
		return "+X"
	case m.DX == -1 && m.DZ == 0:
		return "-X"
	case m.DX == 0 && m.DZ == 1:
		return "+Z"
	case m.DX == 0 && m.DZ == -1:
		return "-Z"
	default:
		return "?"
	}
}

func yawToDir(yaw int) (dx, dz int) {
	y := yaw % 360
	if y < 0 {
		y += 360
	}
	// Nearest 90 degrees.
	dir := ((y + 45) / 90) % 4
	switch dir {
	case 0:
		return 0, 1 // +Z
	case 1:
		return 1, 0 // +X
	case 2:
		return 0, -1 // -Z
	default:
		return -1, 0 // -X
	}
}

func switchIDAt(pos Vec3i) string { return ids.SwitchIDAt(pos.X, pos.Y, pos.Z) }

func (w *World) ensureSwitch(pos Vec3i, on bool) {
	if w.switches == nil {
		w.switches = map[Vec3i]bool{}
	}
	w.switches[pos] = on
}

func (w *World) removeSwitch(nowTick uint64, actor string, pos Vec3i, reason string) {
	if w.switches == nil {
		return
	}
	if _, ok := w.switches[pos]; !ok {
		return
	}
	delete(w.switches, pos)
	w.auditEvent(nowTick, actor, "SWITCH_REMOVE", pos, reason, map[string]any{
		"switch_id": switchIDAt(pos),
	})
}

func (w *World) sortedSwitchPositionsNear(pos Vec3i, dist int) []Vec3i {
	if len(w.switches) == 0 {
		return nil
	}
	out := make([]Vec3i, 0, 8)
	for p := range w.switches {
		if Manhattan(p, pos) > dist {
			continue
		}
		// Guard against stale meta.
		if w.blockName(w.chunks.GetBlock(p)) != "SWITCH" {
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
