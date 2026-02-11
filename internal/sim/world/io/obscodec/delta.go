package obscodec

import "voxelcraft.ai/internal/protocol"

// BuildDeltaOps compares voxel palettes in stable scan order and emits
// protocol delta ops. The caller controls scan order by passing slices
// that were filled using the world's canonical dy/dz/dx order.
func BuildDeltaOps(prev, curr []uint16, radius int) []protocol.VoxelDeltaOp {
	if len(prev) != len(curr) || len(curr) == 0 {
		return nil
	}
	r := radius
	ops := make([]protocol.VoxelDeltaOp, 0, 64)
	i := 0
	for dy := -r; dy <= r; dy++ {
		for dz := -r; dz <= r; dz++ {
			for dx := -r; dx <= r; dx++ {
				if curr[i] != prev[i] {
					ops = append(ops, protocol.VoxelDeltaOp{D: [3]int{dx, dy, dz}, B: curr[i]})
				}
				i++
			}
		}
	}
	return ops
}
