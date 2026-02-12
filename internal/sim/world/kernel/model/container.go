package model

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/world/logic/ids"
)

// Container is the authoritative inventory state for blocks like CHEST/FURNACE/CONTRACT_TERMINAL.
// It is included in snapshots/digests.
type Container struct {
	Type string
	Pos  Vec3i

	Inventory map[string]int
	Reserved  map[string]int            // escrow-reserved (reward/deposit)
	Owed      map[string]map[string]int // agent_id -> item -> count
}

func (c *Container) ID() string { return ContainerID(c.Type, c.Pos) }

func ContainerID(typ string, pos Vec3i) string {
	return ids.ContainerID(typ, pos.X, pos.Y, pos.Z)
}

func ParseContainerID(id string) (typ string, pos Vec3i, ok bool) {
	typ, x, y, z, ok := ids.ParseContainerID(id)
	if !ok {
		return "", Vec3i{}, false
	}
	return typ, Vec3i{X: x, Y: y, Z: z}, true
}

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

func (c *Container) ReservedCount(item string) int {
	if c.Reserved == nil {
		return 0
	}
	return c.Reserved[item]
}

func (c *Container) AvailableCount(item string) int {
	return c.Inventory[item] - c.ReservedCount(item)
}

func (c *Container) Reserve(item string, n int) {
	if n <= 0 {
		return
	}
	if c.Reserved == nil {
		c.Reserved = map[string]int{}
	}
	c.Reserved[item] += n
}

func (c *Container) Unreserve(item string, n int) {
	if n <= 0 || c.Reserved == nil {
		return
	}
	c.Reserved[item] -= n
	if c.Reserved[item] <= 0 {
		delete(c.Reserved, item)
	}
}

func (c *Container) AddOwed(agentID, item string, n int) {
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

func (c *Container) ClaimOwed(agentID string) map[string]int {
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

