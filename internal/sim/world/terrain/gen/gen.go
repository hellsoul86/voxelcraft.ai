package gen

import "voxelcraft.ai/internal/sim/world/logic/mathx"

func FloorDiv(a, b int) int {
	return mathx.FloorDiv(a, b)
}

func Mod(a, b int) int {
	return mathx.Mod(a, b)
}

func Hash2(seed int64, x, z int) uint64 {
	return mathx.Hash2(seed, x, z)
}

func Hash3(seed int64, x, y, z int) uint64 {
	return mathx.Hash3(seed, x, y, z)
}

func BiomeFrom(noise uint64) string {
	switch noise % 3 {
	case 0:
		return "PLAINS"
	case 1:
		return "FOREST"
	default:
		return "DESERT"
	}
}

func BiomeAt(seed int64, x, z, regionSize int) string {
	if regionSize <= 0 {
		regionSize = 1
	}
	rx := FloorDiv(x, regionSize)
	rz := FloorDiv(z, regionSize)
	return BiomeFrom(Hash2(seed, rx, rz))
}

func WithinSpawnClear(x, z, radius int) bool {
	if radius <= 0 {
		return false
	}
	r := int64(radius)
	dx := int64(x)
	dz := int64(z)
	return dx*dx+dz*dz <= r*r
}

func ClampPermille(v int) int {
	if v < 0 {
		return 0
	}
	if v > 1000 {
		return 1000
	}
	return v
}

func ScalePermille(base uint64, scalePermille int) uint64 {
	if scalePermille <= 0 {
		scalePermille = 1000
	}
	scaled := (base*uint64(scalePermille) + 500) / 1000
	if scaled > 1000 {
		return 1000
	}
	return scaled
}

func InCluster(seed int64, x, z, grid, radius int, probPermille uint64) bool {
	if grid <= 0 || radius <= 0 || probPermille == 0 {
		return false
	}
	gx := FloorDiv(x, grid)
	gz := FloorDiv(z, grid)
	r2 := radius * radius

	for dz := -1; dz <= 1; dz++ {
		for dx := -1; dx <= 1; dx++ {
			cgx := gx + dx
			cgz := gz + dz
			h := Hash2(seed, cgx, cgz)
			if h%1000 >= probPermille {
				continue
			}

			ox := int((h >> 10) % uint64(grid))
			oz := int((h >> 20) % uint64(grid))
			cx := cgx*grid + ox
			cz := cgz*grid + oz

			ddx := x - cx
			ddz := z - cz
			if ddx*ddx+ddz*ddz <= r2 {
				return true
			}
		}
	}
	return false
}
