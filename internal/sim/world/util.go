package world

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
)

func sortItemStacks(stacks []protocol.ItemStack) {
	sort.Slice(stacks, func(i, j int) bool { return stacks[i].Item < stacks[j].Item })
}
