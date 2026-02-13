package stream

import (
	"testing"

	chunkspkg "voxelcraft.ai/internal/sim/world/feature/observer/chunks"
)

func TestStepChunkRuntime_PatchFlow(t *testing.T) {
	states := map[ChunkKey]*ChunkRuntimeState{
		{CX: 0, CZ: 0}: {
			LastWantedTick: 1,
			SentFull:       true,
			NeedsFull:      false,
			Surface:        make([]chunkspkg.SurfaceCell, 16*16),
		},
	}
	var patched bool
	StepChunkRuntime(states, ChunkStepInput{
		NowTick:         2,
		Connected:       []ChunkKey{{CX: 0, CZ: 0}},
		Radius:          1,
		MaxChunks:       16,
		MaxFullPerTick:  1,
		EvictAfterTicks: 10,
		Audits: []AuditEntry{{
			Action: "SET_BLOCK",
			Pos:    [3]int{1, 0, 1},
			To:     5,
		}},
	}, ChunkStepDeps{
		ComputeSurface: func(cx, cz int) []chunkspkg.SurfaceCell { return make([]chunkspkg.SurfaceCell, 16*16) },
		ComputeSurfaceAt: func(wx, wz int) chunkspkg.SurfaceCell {
			return chunkspkg.SurfaceCell{B: 5, Y: 0}
		},
		SendFull: func(key ChunkKey, surface []chunkspkg.SurfaceCell) bool { return true },
		SendPatch: func(key ChunkKey, cells []ChunkPatchCell) bool {
			patched = len(cells) == 1 && cells[0].Block == 5
			return true
		},
		SendEvict: func(key ChunkKey) bool { return true },
	})
	if !patched {
		t.Fatalf("expected patch callback to fire")
	}
}

func TestStepVoxelRuntime_DisableEvicts(t *testing.T) {
	states := map[ChunkKey]*VoxelRuntimeState{
		{CX: 0, CZ: 0}: {SentFull: true, Blocks: make([]uint16, 16*16)},
	}
	evicted := 0
	StepVoxelRuntime(states, VoxelStepInput{
		NowTick:   10,
		Enabled:   false,
		Radius:    1,
		MaxChunks: 4,
	}, VoxelStepDeps{
		ComputeVoxels: func(cx, cz int) []uint16 { return make([]uint16, 16*16) },
		SendFull:      func(key ChunkKey, blocks []uint16) bool { return true },
		SendPatch:     func(key ChunkKey, cells []VoxelPatchCell) bool { return true },
		SendEvict: func(key ChunkKey) bool {
			evicted++
			return true
		},
	})
	if evicted != 1 {
		t.Fatalf("evicted=%d want 1", evicted)
	}
	if len(states) != 0 {
		t.Fatalf("states should be cleared when disabled")
	}
}
