package world

import (
	"voxelcraft.ai/internal/protocol"
	simenc "voxelcraft.ai/internal/sim/encoding"
)

func (w *World) buildObsVoxels(center Vec3i, cl *clientState) (protocol.VoxelsObs, []Vec3i) {
	r := w.cfg.ObsRadius
	sensorBlock, hasSensor := w.catalogs.Blocks.Index["SENSOR"]
	sensorsNear := make([]Vec3i, 0, 4)
	dim := 2*r + 1
	plane := dim * dim
	total := plane * dim
	curr := make([]uint16, total)

	// 2D world optimization: only the y==0 slice can be non-AIR; all other y are read-as-AIR.
	// Keep the same scan order (dy outer, dz middle, dx inner) so DELTA ops remain stable.
	air := w.chunks.gen.Air
	if air != 0 {
		for i := range curr {
			curr[i] = air
		}
	}
	// Fill only the slice where world Y equals 0.
	dy0 := -center.Y
	if dy0 >= -r && dy0 <= r {
		layerOff := (dy0 + r) * plane
		for dz := -r; dz <= r; dz++ {
			rowOff := layerOff + (dz+r)*dim
			for dx := -r; dx <= r; dx++ {
				p := Vec3i{X: center.X + dx, Y: 0, Z: center.Z + dz}
				b := w.chunks.GetBlock(p)
				curr[rowOff+(dx+r)] = b
				if hasSensor && b == sensorBlock {
					sensorsNear = append(sensorsNear, p)
				}
			}
		}
	}

	vox := protocol.VoxelsObs{
		Center:   center.ToArray(),
		Radius:   r,
		Encoding: "RLE",
	}

	if cl.DeltaVoxels && cl.LastVoxels != nil && len(cl.LastVoxels) == len(curr) {
		ops := make([]protocol.VoxelDeltaOp, 0, 64)
		i := 0
		for dy := -r; dy <= r; dy++ {
			for dz := -r; dz <= r; dz++ {
				for dx := -r; dx <= r; dx++ {
					if curr[i] != cl.LastVoxels[i] {
						ops = append(ops, protocol.VoxelDeltaOp{D: [3]int{dx, dy, dz}, B: curr[i]})
					}
					i++
				}
			}
		}
		if len(ops) > 0 && len(ops) < len(curr)/2 {
			vox.Encoding = "DELTA"
			vox.Ops = ops
		} else {
			vox.Data = simenc.EncodeRLE(curr)
		}
	} else {
		vox.Data = simenc.EncodeRLE(curr)
	}
	cl.LastVoxels = curr

	return vox, sensorsNear
}
