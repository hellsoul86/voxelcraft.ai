package stream

import chunkspkg "voxelcraft.ai/internal/sim/world/feature/observer/chunks"

type ChunkClientInput struct {
	NowTick   uint64
	Connected []ChunkKey
	Audits    []AuditEntry
}

type ChunkClientDeps struct {
	ComputeSurface   func(cx, cz int) []chunkspkg.SurfaceCell
	ComputeSurfaceAt func(wx, wz int) chunkspkg.SurfaceCell
	SendFull         func(key ChunkKey, surface []chunkspkg.SurfaceCell) bool
	SendPatch        func(key ChunkKey, cells []ChunkPatchCell) bool
	SendEvict        func(key ChunkKey) bool
}

func StepChunkClient(c *Client, in ChunkClientInput, deps ChunkClientDeps) {
	if c == nil {
		return
	}
	states := make(map[ChunkKey]*ChunkRuntimeState, len(c.Chunks))
	for k, st := range c.Chunks {
		states[k] = &ChunkRuntimeState{
			LastWantedTick: st.LastWantedTick,
			SentFull:       st.SentFull,
			NeedsFull:      st.NeedsFull,
			Surface:        st.Surface,
		}
	}

	StepChunkRuntime(states, ChunkStepInput{
		NowTick:         in.NowTick,
		Connected:       in.Connected,
		Radius:          c.Config.ChunkRadius,
		MaxChunks:       c.Config.MaxChunks,
		MaxFullPerTick:  ObserverMaxFullChunksPerTick,
		EvictAfterTicks: ObserverEvictAfterTicks,
		Audits:          in.Audits,
	}, ChunkStepDeps{
		ComputeSurface:   deps.ComputeSurface,
		ComputeSurfaceAt: deps.ComputeSurfaceAt,
		SendFull:         deps.SendFull,
		SendPatch:        deps.SendPatch,
		SendEvict:        deps.SendEvict,
	})

	next := make(map[ChunkKey]*ChunkState, len(states))
	for k, st := range states {
		next[k] = &ChunkState{
			Key:            k,
			LastWantedTick: st.LastWantedTick,
			SentFull:       st.SentFull,
			NeedsFull:      st.NeedsFull,
			Surface:        st.Surface,
		}
	}
	c.Chunks = next
}

type VoxelClientInput struct {
	NowTick      uint64
	Enabled      bool
	FocusCenters []ChunkKey
	Audits       []AuditEntry
}

type VoxelClientDeps struct {
	ComputeVoxels func(cx, cz int) []uint16
	SendFull      func(key ChunkKey, blocks []uint16) bool
	SendPatch     func(key ChunkKey, cells []VoxelPatchCell) bool
	SendEvict     func(key ChunkKey) bool
}

func StepVoxelClient(c *Client, in VoxelClientInput, deps VoxelClientDeps) {
	if c == nil {
		return
	}
	states := make(map[ChunkKey]*VoxelRuntimeState, len(c.VoxelChunks))
	for k, st := range c.VoxelChunks {
		blocks := make([]uint16, len(st.Blocks))
		copy(blocks, st.Blocks)
		states[k] = &VoxelRuntimeState{
			LastWantedTick: st.LastWantedTick,
			SentFull:       st.SentFull,
			NeedsFull:      st.NeedsFull,
			Blocks:         blocks,
		}
	}

	StepVoxelRuntime(states, VoxelStepInput{
		NowTick:         in.NowTick,
		Enabled:         in.Enabled,
		FocusCenters:    in.FocusCenters,
		Radius:          c.Config.VoxelRadius,
		MaxChunks:       c.Config.VoxelMaxChunks,
		MaxFullPerTick:  ObserverMaxFullVoxelChunksPerTick,
		EvictAfterTicks: ObserverVoxelEvictAfterTicks,
		Audits:          in.Audits,
	}, VoxelStepDeps{
		ComputeVoxels: deps.ComputeVoxels,
		SendFull:      deps.SendFull,
		SendPatch:     deps.SendPatch,
		SendEvict:     deps.SendEvict,
	})

	next := make(map[ChunkKey]*VoxelState, len(states))
	for k, st := range states {
		blocks := make([]uint16, len(st.Blocks))
		copy(blocks, st.Blocks)
		next[k] = &VoxelState{
			Key:            k,
			LastWantedTick: st.LastWantedTick,
			SentFull:       st.SentFull,
			NeedsFull:      st.NeedsFull,
			Blocks:         blocks,
		}
	}
	c.VoxelChunks = next
}
