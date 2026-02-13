package respawn

import modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"

// ResetForSeason resets per-season physical/progression state while preserving
// long-term identity fields (memory/reputation/org) on the agent.
func ResetForSeason(a *modelpkg.Agent, findSpawnAir func(x, z, maxRadius int) (int, int)) {
	if a == nil {
		return
	}
	// Cancel ongoing tasks.
	a.MoveTask = nil
	a.WorkTask = nil

	// Reset physical attributes.
	a.HP = 20
	a.Hunger = 20
	a.StaminaMilli = 1000
	a.Yaw = 0

	// Reset inventory to starter kit.
	a.Inventory = map[string]int{
		"PLANK":   20,
		"COAL":    10,
		"STONE":   20,
		"BERRIES": 10,
	}

	// Reset equipment (MVP).
	a.Equipment = modelpkg.Equipment{MainHand: "NONE", Armor: [4]string{"NONE", "NONE", "NONE", "NONE"}}

	// Clear ephemeral queues.
	a.Events = nil
	a.PendingMemory = nil

	// Reset anti-exploit windows so novelty/fun can be earned per season.
	a.ResetRateLimits()
	a.ResetFunTracking()
	a.Fun = modelpkg.FunScore{}

	// Respawn at deterministic spawn point.
	n := AgentNumber(a.ID)
	spawnXZ := n * 2
	spawnX := spawnXZ
	spawnZ := -spawnXZ
	if findSpawnAir != nil {
		spawnX, spawnZ = findSpawnAir(spawnX, spawnZ, 8)
	}
	a.Pos = modelpkg.Vec3i{X: spawnX, Y: 0, Z: spawnZ}
}
