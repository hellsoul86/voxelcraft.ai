package world

import statspkg "voxelcraft.ai/internal/sim/world/feature/director/stats"

type StatsBucket = statspkg.Bucket
type StatsChunkKey = statspkg.ChunkKey
type WorldStats = statspkg.WorldStats

func NewWorldStats(bucketTicks, windowTicks uint64) *WorldStats {
	return statspkg.NewWorldStats(bucketTicks, windowTicks)
}
