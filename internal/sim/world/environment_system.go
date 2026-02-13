package world

import (
	"sort"

	respawnpkg "voxelcraft.ai/internal/sim/world/feature/survival/respawn"
	survivalruntimepkg "voxelcraft.ai/internal/sim/world/feature/survival/runtime"
)

func (w *World) systemEnvironment(nowTick uint64) {
	survivalruntimepkg.Tick(
		survivalruntimepkg.TickInput{
			NowTick:           nowTick,
			Agents:            w.sortedAgents(),
			Weather:           w.weather,
			ActiveEventID:     w.activeEventID,
			ActiveEventCenter: w.activeEventCenter,
			ActiveEventRadius: w.activeEventRadius,
			ActiveEventEnds:   w.activeEventEnds,
		},
		survivalruntimepkg.TickHooks{
			TimeOfDay:      w.timeOfDay,
			DistXZ:         distXZ,
			NearBlock:      w.nearBlock,
			Respawn:        w.respawnAgent,
			CleanupExpired: w.cleanupExpiredItemEntities,
		},
	)
}

func (w *World) respawnAgent(nowTick uint64, a *Agent, reason string) {
	respawnpkg.Apply(nowTick, a, reason, respawnpkg.Hooks{
		SpawnItem:    w.spawnItemEntity,
		FindSpawnAir: w.findSpawnAir,
	})
}

func (w *World) sortedAgents() []*Agent {
	out := make([]*Agent, 0, len(w.agents))
	for _, a := range w.agents {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
