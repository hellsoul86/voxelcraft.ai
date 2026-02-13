package chunks

import (
	"sort"

	"voxelcraft.ai/internal/sim/world/logic/mathx"
)

type Key struct {
	CX int
	CZ int
}

type SurfaceCell struct {
	B uint16
	Y uint8
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

func ComputeChunkSurface(blocks []uint16, cx, cz int, air uint16, boundaryR int) []SurfaceCell {
	out := make([]SurfaceCell, 16*16)
	for z := 0; z < 16; z++ {
		for x := 0; x < 16; x++ {
			wx := cx*16 + x
			wz := cz*16 + z
			if boundaryR > 0 && (wx < -boundaryR || wx > boundaryR || wz < -boundaryR || wz > boundaryR) {
				out[z*16+x] = SurfaceCell{B: air, Y: 0}
				continue
			}
			b := air
			if idx := x + z*16; idx >= 0 && idx < len(blocks) {
				v := blocks[idx]
				if v != air {
					b = v
				}
			}
			out[z*16+x] = SurfaceCell{B: b, Y: 0}
		}
	}
	return out
}

func ComputeChunkVoxels(blocks []uint16, cx, cz int, air uint16, boundaryR int) []uint16 {
	if blocks == nil {
		return nil
	}
	out := make([]uint16, len(blocks))
	copy(out, blocks)
	if boundaryR <= 0 {
		return out
	}
	for z := 0; z < 16; z++ {
		for x := 0; x < 16; x++ {
			wx := cx*16 + x
			wz := cz*16 + z
			if wx < -boundaryR || wx > boundaryR || wz < -boundaryR || wz > boundaryR {
				out[x+z*16] = air
			}
		}
	}
	return out
}

func ComputeSurfaceCellAt(wx, wz int, air uint16, boundaryR int, chunkBlocks func(cx, cz int) []uint16) SurfaceCell {
	if boundaryR > 0 && (wx < -boundaryR || wx > boundaryR || wz < -boundaryR || wz > boundaryR) {
		return SurfaceCell{B: air, Y: 0}
	}
	cx := floorDiv(wx, 16)
	cz := floorDiv(wz, 16)
	lx := mod(wx, 16)
	lz := mod(wz, 16)
	blocks := chunkBlocks(cx, cz)
	idx := lx + lz*16
	if idx >= 0 && idx < len(blocks) {
		v := blocks[idx]
		if v != air {
			return SurfaceCell{B: v, Y: 0}
		}
	}
	return SurfaceCell{B: air, Y: 0}
}

func floorDiv(a, b int) int {
	if b == 0 {
		return 0
	}
	q := a / b
	r := a % b
	if r != 0 && ((r > 0) != (b > 0)) {
		q--
	}
	return q
}

func mod(a, b int) int {
	if b == 0 {
		return 0
	}
	r := a % b
	if r < 0 {
		r += b
	}
	return r
}
