package world

import (
	"fmt"
	"strconv"
	"strings"

	"voxelcraft.ai/internal/protocol"
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
	sortItemStacks(out)
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
	return fmt.Sprintf("%s@%d,%d,%d", typ, pos.X, pos.Y, pos.Z)
}

func parseContainerID(id string) (typ string, pos Vec3i, ok bool) {
	parts := strings.SplitN(id, "@", 2)
	if len(parts) != 2 {
		return "", Vec3i{}, false
	}
	typ = parts[0]
	coord := strings.Split(parts[1], ",")
	if len(coord) != 3 {
		return "", Vec3i{}, false
	}
	x, err1 := strconv.Atoi(coord[0])
	y, err2 := strconv.Atoi(coord[1])
	z, err3 := strconv.Atoi(coord[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return "", Vec3i{}, false
	}
	return typ, Vec3i{X: x, Y: y, Z: z}, true
}
