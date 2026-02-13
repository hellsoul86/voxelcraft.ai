package respawn

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type SpawnItemFn func(nowTick uint64, actor string, pos modelpkg.Vec3i, item string, count int, reason string) string
type FindSpawnAirFn func(x, z, radius int) (int, int)

type Hooks struct {
	SpawnItem    SpawnItemFn
	FindSpawnAir FindSpawnAirFn
}

func Apply(nowTick uint64, a *modelpkg.Agent, reason string, hooks Hooks) {
	if a == nil {
		return
	}

	// Cancel ongoing tasks.
	a.MoveTask = nil
	a.WorkTask = nil

	// Drop ~30% of each stack (deterministic) at the downed position.
	dropPos := a.Pos
	lost := ComputeInventoryLoss(a.Inventory)

	// Spawn dropped items as world item entities.
	if hooks.SpawnItem != nil && len(lost) > 0 {
		keys := make([]string, 0, len(lost))
		for k := range lost {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			n := lost[k]
			if n <= 0 {
				continue
			}
			_ = hooks.SpawnItem(nowTick, a.ID, dropPos, k, n, "RESPAWN_DROP")
		}
	}

	// Respawn at a stable spawn point near origin.
	n := AgentNumber(a.ID)
	spawnXZ := n * 2
	spawnX := spawnXZ
	spawnZ := -spawnXZ
	if hooks.FindSpawnAir != nil {
		spawnX, spawnZ = hooks.FindSpawnAir(spawnX, spawnZ, 8)
	}
	a.Pos = modelpkg.Vec3i{X: spawnX, Y: 0, Z: spawnZ}
	a.Yaw = 0

	a.HP = 20
	a.Hunger = 10
	a.StaminaMilli = 1000

	ev := protocol.Event{
		"t":        nowTick,
		"type":     "RESPAWN",
		"reason":   reason,
		"pos":      a.Pos.ToArray(),
		"drop_pos": dropPos.ToArray(),
	}
	if len(lost) > 0 {
		ev["lost"] = inventorypkg.EncodeItemPairs(lost)
	}
	a.AddEvent(ev)
}
