package world

import (
	"voxelcraft.ai/internal/sim/world/logic/mathx"
	genpkg "voxelcraft.ai/internal/sim/world/terrain/gen"
)

func (w *World) surfaceY(x, z int) int {
	// Pure 2D world: the only valid y coordinate is 0.
	_ = x
	_ = z
	return 0
}

func (w *World) findSpawnAir(x, z int, maxR int) (int, int) {
	if w == nil || w.chunks == nil {
		return x, z
	}
	if maxR < 0 {
		maxR = 0
	}
	air := w.chunks.gen.Air
	for r := 0; r <= maxR; r++ {
		for dz := -r; dz <= r; dz++ {
			for dx := -r; dx <= r; dx++ {
				// Check the perimeter only (square spiral) for deterministic order.
				if mathx.AbsInt(dx) != r && mathx.AbsInt(dz) != r {
					continue
				}
				px := x + dx
				pz := z + dz
				p := Vec3i{X: px, Y: 0, Z: pz}
				if !w.chunks.inBounds(p) {
					continue
				}
				if w.chunks.GetBlock(p) == air {
					return px, pz
				}
			}
		}
	}
	return x, z
}

func (w *World) nearBlock(pos Vec3i, blockID string, dist int) bool {
	bid, ok := w.catalogs.Blocks.Index[blockID]
	if !ok {
		return false
	}
	for dy := -dist; dy <= dist; dy++ {
		for dz := -dist; dz <= dist; dz++ {
			for dx := -dist; dx <= dist; dx++ {
				p := Vec3i{X: pos.X + dx, Y: pos.Y + dy, Z: pos.Z + dz}
				if w.chunks.GetBlock(p) == bid {
					return true
				}
			}
		}
	}
	return false
}

func (w *World) blockIDToItem(b uint16) string {
	if int(b) < 0 || int(b) >= len(w.catalogs.Blocks.Palette) {
		return ""
	}
	blockName := w.catalogs.Blocks.Palette[b]
	// If an item with same id exists, drop that.
	if _, ok := w.catalogs.Items.Defs[blockName]; ok {
		return blockName
	}
	// Special: ore blocks drop the ore item id.
	switch blockName {
	case "COAL_ORE":
		return "COAL"
	case "IRON_ORE":
		return "IRON_ORE"
	case "COPPER_ORE":
		return "COPPER_ORE"
	case "CRYSTAL_ORE":
		return "CRYSTAL_SHARD"
	}
	return ""
}

func (w *World) blockName(b uint16) string {
	if int(b) < 0 || int(b) >= len(w.catalogs.Blocks.Palette) {
		return ""
	}
	return w.catalogs.Blocks.Palette[b]
}

func (w *World) blockSolid(b uint16) bool {
	name := w.blockName(b)
	if name == "" {
		return true
	}
	def, ok := w.catalogs.Blocks.Defs[name]
	if !ok {
		return true
	}
	return def.Solid
}

func floorDiv(a, b int) int                   { return genpkg.FloorDiv(a, b) }
func mod(a, b int) int                        { return genpkg.Mod(a, b) }
func hash2(seed int64, x, z int) uint64       { return genpkg.Hash2(seed, x, z) }
func hash3(seed int64, x, y, z int) uint64    { return genpkg.Hash3(seed, x, y, z) }
func biomeFrom(noise uint64) string           { return genpkg.BiomeFrom(noise) }
func biomeAt(seed int64, x, z, r int) string  { return genpkg.BiomeAt(seed, x, z, r) }
func withinSpawnClear(x, z, radius int) bool  { return genpkg.WithinSpawnClear(x, z, radius) }
func clampPermille(v int) int                 { return genpkg.ClampPermille(v) }
func scalePermille(base uint64, p int) uint64 { return genpkg.ScalePermille(base, p) }
func inCluster(seed int64, x, z, grid, radius int, probPermille uint64) bool {
	return genpkg.InCluster(seed, x, z, grid, radius, probPermille)
}
