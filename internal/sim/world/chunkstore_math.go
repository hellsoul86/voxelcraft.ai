package world

import "voxelcraft.ai/internal/sim/world/logic/mathx"

func floorDiv(a, b int) int {
	return mathx.FloorDiv(a, b)
}

func mod(a, b int) int {
	return mathx.Mod(a, b)
}

func hash2(seed int64, x, z int) uint64 {
	return mathx.Hash2(seed, x, z)
}

func hash3(seed int64, x, y, z int) uint64 {
	return mathx.Hash3(seed, x, y, z)
}

func biomeFrom(noise uint64) string {
	// 3-way split.
	switch noise % 3 {
	case 0:
		return "PLAINS"
	case 1:
		return "FOREST"
	default:
		return "DESERT"
	}
}

func biomeAt(seed int64, x, z, regionSize int) string {
	if regionSize <= 0 {
		regionSize = 1
	}
	rx := floorDiv(x, regionSize)
	rz := floorDiv(z, regionSize)
	return biomeFrom(hash2(seed, rx, rz))
}

func withinSpawnClear(x, z, radius int) bool {
	if radius <= 0 {
		return false
	}
	r := int64(radius)
	dx := int64(x)
	dz := int64(z)
	return dx*dx+dz*dz <= r*r
}

func clampPermille(v int) int {
	if v < 0 {
		return 0
	}
	if v > 1000 {
		return 1000
	}
	return v
}

func scalePermille(base uint64, scalePermille int) uint64 {
	if scalePermille <= 0 {
		scalePermille = 1000
	}
	// Nearest integer rounding: (base*scale + 500)/1000
	scaled := (base*uint64(scalePermille) + 500) / 1000
	if scaled > 1000 {
		return 1000
	}
	return scaled
}

func inCluster(seed int64, x, z, grid, radius int, probPermille uint64) bool {
	if grid <= 0 || radius <= 0 || probPermille == 0 {
		return false
	}
	gx := floorDiv(x, grid)
	gz := floorDiv(z, grid)
	r2 := radius * radius

	for dz := -1; dz <= 1; dz++ {
		for dx := -1; dx <= 1; dx++ {
			cgx := gx + dx
			cgz := gz + dz
			h := hash2(seed, cgx, cgz)
			if h%1000 >= probPermille {
				continue
			}

			// Deterministically place a center inside this grid cell.
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
