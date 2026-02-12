package chunks

import (
	"sort"

	"voxelcraft.ai/internal/sim/world/logic/mathx"
)

type Key struct {
	CX int
	CZ int
}

func ComputeWantedChunks(agents []Key, radius int, maxChunks int) []Key {
	if radius <= 0 {
		radius = 1
	}
	if maxChunks <= 0 {
		maxChunks = 1024
	}
	type item struct {
		k    Key
		dist int
	}
	distByKey := map[Key]int{}
	for _, a := range agents {
		for dz := -radius; dz <= radius; dz++ {
			for dx := -radius; dx <= radius; dx++ {
				k := Key{CX: a.CX + dx, CZ: a.CZ + dz}
				d := mathx.AbsInt(dx) + mathx.AbsInt(dz)
				if prev, ok := distByKey[k]; !ok || d < prev {
					distByKey[k] = d
				}
			}
		}
	}
	items := make([]item, 0, len(distByKey))
	for k, d := range distByKey {
		items = append(items, item{k: k, dist: d})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].dist != items[j].dist {
			return items[i].dist < items[j].dist
		}
		if items[i].k.CX != items[j].k.CX {
			return items[i].k.CX < items[j].k.CX
		}
		return items[i].k.CZ < items[j].k.CZ
	})
	if len(items) > maxChunks {
		items = items[:maxChunks]
	}
	out := make([]Key, 0, len(items))
	for _, it := range items {
		out = append(out, it.k)
	}
	return out
}

func ClampInt(v, min, max, def int) int {
	if v == 0 {
		v = def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func Ceil(v float64) float64 {
	i := int(v)
	if v == float64(i) {
		return v
	}
	if v > 0 {
		return float64(i + 1)
	}
	return float64(i)
}
