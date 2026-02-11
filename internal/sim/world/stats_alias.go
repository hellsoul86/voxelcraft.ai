package world

import featuredirector "voxelcraft.ai/internal/sim/world/feature/director"

type StatsBucket = featuredirector.StatsBucket
type StatsChunkKey = featuredirector.StatsChunkKey
type WorldStats = featuredirector.WorldStats

func NewWorldStats(bucketTicks, windowTicks uint64) *WorldStats {
	return featuredirector.NewWorldStats(bucketTicks, windowTicks)
}
