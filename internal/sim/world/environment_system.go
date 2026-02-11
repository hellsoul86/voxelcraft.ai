package world

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
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
				a.Hunger--
				// Event hazard: blight zones increase hunger drain.
				if w.activeEventID == "BLIGHT_ZONE" && w.activeEventRadius > 0 && nowTick < w.activeEventEnds &&
					distXZ(a.Pos, w.activeEventCenter) <= w.activeEventRadius {
					a.Hunger--
					if a.Hunger < 0 {
						a.Hunger = 0
					}
				}
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
		t := w.timeOfDay(nowTick)
		isNight := t < 0.25 || t > 0.75
		if isNight {
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

		// Stamina recovery: faster when fed, slower during storms/cold.
		rec := 2
		if w.weather == "STORM" {
			rec = 1
		}
		if w.weather == "COLD" {
			rec = 1
		}
		if a.Hunger == 0 {
			rec = 0
		} else if a.Hunger < 5 && rec > 1 {
			rec = 1
		}

		// Event hazards.
		if w.activeEventID != "" && w.activeEventRadius > 0 && nowTick < w.activeEventEnds &&
			distXZ(a.Pos, w.activeEventCenter) <= w.activeEventRadius {
			switch w.activeEventID {
			case "BLIGHT_ZONE":
				rec = 0
			case "FLOOD_WARNING":
				if rec > 1 {
					rec = 1
				}
			}
		}

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
	lost := map[string]int{}
	if len(a.Inventory) > 0 {
		keys := make([]string, 0, len(a.Inventory))
		for k, n := range a.Inventory {
			if k != "" && n > 0 {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			n := a.Inventory[k]
			d := (n * 3) / 10
			if d <= 0 {
				continue
			}
			a.Inventory[k] -= d
			if a.Inventory[k] <= 0 {
				delete(a.Inventory, k)
			}
			lost[k] = d
		}
		if len(lost) == 0 {
			// Ensure at least something is lost if inventory is non-empty.
			for _, k := range keys {
				if a.Inventory[k] > 0 {
					a.Inventory[k]--
					if a.Inventory[k] <= 0 {
						delete(a.Inventory, k)
					}
					lost[k] = 1
					break
				}
			}
		}
	}

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
	n := agentNum(a.ID)
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
		ev["lost"] = encodeItemPairs(lost)
	}
	a.AddEvent(ev)
}

func agentNum(agentID string) int {
	if len(agentID) < 2 || agentID[0] != 'A' {
		return 0
	}
	n := 0
	for i := 1; i < len(agentID); i++ {
		c := agentID[i]
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func (w *World) sortedAgents() []*Agent {
	out := make([]*Agent, 0, len(w.agents))
	for _, a := range w.agents {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
