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
