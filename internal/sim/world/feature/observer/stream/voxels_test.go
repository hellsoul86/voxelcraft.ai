package stream

import (
	"testing"

	simenc "voxelcraft.ai/internal/sim/encoding"
)

func TestBuildObsVoxels2D_RLE(t *testing.T) {
	in := VoxelsBuildInput{
		Center:      VoxelPos{X: 10, Y: 0, Z: 20},
		Radius:      1,
		AirBlock:    0,
		HasSensor:   true,
		SensorBlock: 7,
	}
	out := BuildObsVoxels2D(in, func(pos VoxelPos) uint16 {
		if pos.X == 10 && pos.Z == 20 {
			return 7
		}
		return 3
	})
	if out.Voxels.Encoding != "RLE" {
		t.Fatalf("encoding=%q want RLE", out.Voxels.Encoding)
	}
	if len(out.SensorPos) != 1 || out.SensorPos[0] != (VoxelPos{X: 10, Y: 0, Z: 20}) {
		t.Fatalf("sensor positions mismatch: %+v", out.SensorPos)
	}
	decoded, err := simenc.DecodeRLE(out.Voxels.Data)
	if err != nil {
		t.Fatalf("decode rle: %v", err)
	}
	if len(decoded) != 27 {
		t.Fatalf("decoded len=%d want 27", len(decoded))
	}
}

func TestBuildObsVoxels2D_Delta(t *testing.T) {
	// Start with all AIR in a 3x3x3 volume.
	last := make([]uint16, 27)
	in := VoxelsBuildInput{
		Center:       VoxelPos{X: 0, Y: 0, Z: 0},
		Radius:       1,
		AirBlock:     0,
		DeltaEnabled: true,
		LastVoxels:   last,
	}
	out := BuildObsVoxels2D(in, func(pos VoxelPos) uint16 {
		if pos.X == 0 && pos.Z == 0 {
			return 5
		}
		return 0
	})
	if out.Voxels.Encoding != "DELTA" {
		t.Fatalf("encoding=%q want DELTA", out.Voxels.Encoding)
	}
	if len(out.Voxels.Ops) == 0 {
		t.Fatalf("expected non-empty delta ops")
	}
}
