package world

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	respawnpkg "voxelcraft.ai/internal/sim/world/feature/survival/respawn"
	survivalruntimepkg "voxelcraft.ai/internal/sim/world/feature/survival/runtime"
)

func (w *World) systemEnvironment(nowTick uint64) {
	agents := w.sortedAgents()

	// Soft survival: hunger ticks down slowly; low hunger reduces stamina recovery.
	if nowTick%200 == 0 { // ~40s at 5Hz
		for _, a := range agents {
			if a == nil {
				continue
			}
			if a.Hunger > 0 {
				inBlight := w.activeEventID == "BLIGHT_ZONE" && w.activeEventRadius > 0 && nowTick < w.activeEventEnds &&
					distXZ(a.Pos, w.activeEventCenter) <= w.activeEventRadius
				a.Hunger = survivalruntimepkg.HungerAfterTick(a.Hunger, inBlight)
			} else {
				// Starvation pressure (slow, non-lethal alone unless ignored).
				if a.HP > 0 {
					a.HP--
					a.AddEvent(protocol.Event{"t": nowTick, "type": "DAMAGE", "kind": "STARVATION", "hp": a.HP})
				}
			}
		}
	}

	// Weather hazards (minimal): cold snaps hurt at night unless near a torch.
	if w.weather == "COLD" && nowTick%50 == 0 { // ~10s
		if survivalruntimepkg.IsNight(w.timeOfDay(nowTick)) {
			for _, a := range agents {
				if a == nil || a.HP <= 0 {
					continue
				}
				if w.nearBlock(a.Pos, "TORCH", 3) {
					continue
				}
				a.HP--
				a.AddEvent(protocol.Event{"t": nowTick, "type": "DAMAGE", "kind": "COLD", "hp": a.HP})
			}
		}
	}

	// Event hazard: bandit camp is safer in groups.
	banditZoneCount := 0
	if w.activeEventID == "BANDIT_CAMP" && w.activeEventRadius > 0 && nowTick < w.activeEventEnds {
		for _, a := range agents {
			if a == nil || a.HP <= 0 {
				continue
			}
			if distXZ(a.Pos, w.activeEventCenter) <= w.activeEventRadius {
				banditZoneCount++
			}
		}
	}

	for _, a := range agents {
		if a == nil {
			continue
		}

		inEventRadius := w.activeEventID != "" && w.activeEventRadius > 0 && nowTick < w.activeEventEnds &&
			distXZ(a.Pos, w.activeEventCenter) <= w.activeEventRadius
		rec := survivalruntimepkg.StaminaRecovery(w.weather, a.Hunger, w.activeEventID, inEventRadius)

		// Bandit camp damage: when alone, take periodic hits.
		if w.activeEventID == "BANDIT_CAMP" && w.activeEventRadius > 0 && nowTick < w.activeEventEnds &&
			nowTick%50 == 0 && banditZoneCount > 0 && banditZoneCount < 2 &&
			distXZ(a.Pos, w.activeEventCenter) <= w.activeEventRadius && a.HP > 0 {
			a.HP--
			a.AddEvent(protocol.Event{"t": nowTick, "type": "DAMAGE", "kind": "BANDIT", "hp": a.HP})
		}

		if a.StaminaMilli < 1000 && rec > 0 {
			a.StaminaMilli += rec
			if a.StaminaMilli > 1000 {
				a.StaminaMilli = 1000
			}
		}

		// Downed -> respawn.
		if a.HP <= 0 {
			w.respawnAgent(nowTick, a, "DOWNED")
		}
	}

	// Cleanup: despawn expired dropped items (rate-limited to keep per-tick work low).
	if nowTick%50 == 0 {
		w.cleanupExpiredItemEntities(nowTick)
	}
}

func (w *World) respawnAgent(nowTick uint64, a *Agent, reason string) {
	if a == nil {
		return
	}

	// Cancel ongoing tasks.
	a.MoveTask = nil
	a.WorkTask = nil

	// Drop ~30% of each stack (deterministic) at the downed position.
	dropPos := a.Pos
	lost := respawnpkg.ComputeInventoryLoss(a.Inventory)

	// Spawn dropped items as world item entities.
	if len(lost) > 0 {
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
			_ = w.spawnItemEntity(nowTick, a.ID, dropPos, k, n, "RESPAWN_DROP")
		}
	}

	// Respawn at a stable spawn point near origin.
	n := respawnpkg.AgentNumber(a.ID)
	spawnXZ := n * 2
	spawnX := spawnXZ
	spawnZ := -spawnXZ
	spawnX, spawnZ = w.findSpawnAir(spawnX, spawnZ, 8)
	a.Pos = Vec3i{X: spawnX, Y: 0, Z: spawnZ}
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

func (w *World) sortedAgents() []*Agent {
	out := make([]*Agent, 0, len(w.agents))
	for _, a := range w.agents {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
