package chunks

import "testing"

func TestComputeWantedChunks(t *testing.T) {
	out := ComputeWantedChunks([]Key{{CX: 0, CZ: 0}}, 1, 4)
	if len(out) != 4 {
		t.Fatalf("expected clipped 4 chunks, got %d", len(out))
	}
	if out[0] != (Key{CX: 0, CZ: 0}) {
		t.Fatalf("closest chunk should be center, got %+v", out[0])
	}
}

func TestClampAndCeil(t *testing.T) {
	if got := ClampInt(0, 1, 5, 3); got != 3 {
		t.Fatalf("ClampInt default failed: %d", got)
	}
	if got := Ceil(1.1); got != 2 {
		t.Fatalf("Ceil failed: %v", got)
	}
}

func TestComputeChunkSurfaceAndCell(t *testing.T) {
	air := uint16(0)
	stone := uint16(2)
	blocks := make([]uint16, 16*16)
	blocks[3+4*16] = stone

	surface := ComputeChunkSurface(blocks, 0, 0, air, 0)
	if got := surface[3+4*16]; got.B != stone || got.Y != 0 {
		t.Fatalf("surface cell mismatch: %+v", got)
	}

	cell := ComputeSurfaceCellAt(3, 4, air, 0, func(cx, cz int) []uint16 {
		if cx == 0 && cz == 0 {
			return blocks
		}
		return make([]uint16, 16*16)
	})
	if cell.B != stone || cell.Y != 0 {
		t.Fatalf("cell mismatch: %+v", cell)
	}
}

func TestComputeChunkVoxelsBoundary(t *testing.T) {
	air := uint16(0)
	stone := uint16(2)
	blocks := make([]uint16, 16*16)
	for i := range blocks {
		blocks[i] = stone
	}
	// cx=1 means world x in [16..31], out of boundary when boundary=8.
	out := ComputeChunkVoxels(blocks, 1, 0, air, 8)
	for i := range out {
		if out[i] != air {
			t.Fatalf("expected boundary-clamped AIR at idx=%d, got=%d", i, out[i])
		}
	}
}
