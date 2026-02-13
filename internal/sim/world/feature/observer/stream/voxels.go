package stream

import (
	"voxelcraft.ai/internal/protocol"
	simenc "voxelcraft.ai/internal/sim/encoding"
	"voxelcraft.ai/internal/sim/world/io/obscodec"
)

type VoxelPos struct {
	X int
	Y int
	Z int
}

type VoxelsBuildInput struct {
	Center      VoxelPos
	Radius      int
	AirBlock    uint16
	HasSensor   bool
	SensorBlock uint16

	DeltaEnabled bool
	LastVoxels   []uint16
}

type VoxelsBuildOutput struct {
	Voxels    protocol.VoxelsObs
	Current   []uint16
	SensorPos []VoxelPos
}

func BuildObsVoxels2D(in VoxelsBuildInput, getBlock func(pos VoxelPos) uint16) VoxelsBuildOutput {
	r := in.Radius
	dim := 2*r + 1
	plane := dim * dim
	total := plane * dim
	curr := make([]uint16, total)
	sensorsNear := make([]VoxelPos, 0, 4)

	if in.AirBlock != 0 {
		for i := range curr {
			curr[i] = in.AirBlock
		}
	}

	dy0 := -in.Center.Y
	if dy0 >= -r && dy0 <= r {
		layerOff := (dy0 + r) * plane
		for dz := -r; dz <= r; dz++ {
			rowOff := layerOff + (dz+r)*dim
			for dx := -r; dx <= r; dx++ {
				p := VoxelPos{X: in.Center.X + dx, Y: 0, Z: in.Center.Z + dz}
				b := getBlock(p)
				curr[rowOff+(dx+r)] = b
				if in.HasSensor && b == in.SensorBlock {
					sensorsNear = append(sensorsNear, p)
				}
			}
		}
	}

	vox := protocol.VoxelsObs{
		Center:   [3]int{in.Center.X, in.Center.Y, in.Center.Z},
		Radius:   r,
		Encoding: "RLE",
	}

	if in.DeltaEnabled && in.LastVoxels != nil && len(in.LastVoxels) == len(curr) {
		ops := obscodec.BuildDeltaOps(in.LastVoxels, curr, r)
		if len(ops) > 0 && len(ops) < len(curr)/2 {
			vox.Encoding = "DELTA"
			vox.Ops = ops
		} else {
			vox.Data = simenc.EncodeRLE(curr)
		}
	} else {
		vox.Data = simenc.EncodeRLE(curr)
	}

	return VoxelsBuildOutput{
		Voxels:    vox,
		Current:   curr,
		SensorPos: sensorsNear,
	}
}
