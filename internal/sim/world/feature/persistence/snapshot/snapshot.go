package snapshot

import snapshotcodecpkg "voxelcraft.ai/internal/sim/world/io/snapshotcodec"

func PositiveMap(src map[string]int) map[string]int {
	return snapshotcodecpkg.PositiveMap(src)
}

func PositiveNestedMap(src map[string]map[string]int) map[string]map[string]int {
	return snapshotcodecpkg.PositiveNestedMap(src)
}
