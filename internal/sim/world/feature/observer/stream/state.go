package stream

import chunkspkg "voxelcraft.ai/internal/sim/world/feature/observer/chunks"

type Config struct {
	ChunkRadius int
	MaxChunks   int

	FocusAgentID   string
	VoxelRadius    int
	VoxelMaxChunks int
}

type Client struct {
	ID      string
	TickOut chan []byte
	DataOut chan []byte

	Config Config

	Chunks      map[ChunkKey]*ChunkState
	VoxelChunks map[ChunkKey]*VoxelState
}

type ChunkState struct {
	Key            ChunkKey
	LastWantedTick uint64
	SentFull       bool
	NeedsFull      bool
	Surface        []chunkspkg.SurfaceCell
}

type VoxelState struct {
	Key            ChunkKey
	LastWantedTick uint64
	SentFull       bool
	NeedsFull      bool
	Blocks         []uint16
}

const (
	ObserverEvictAfterTicks      uint64 = 50
	ObserverMaxFullChunksPerTick        = 32

	ObserverVoxelEvictAfterTicks      uint64 = 10
	ObserverMaxFullVoxelChunksPerTick        = 8
)
