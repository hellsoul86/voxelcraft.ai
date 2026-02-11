package world

import (
	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/tasks"
)

func (w *World) importSnapshotAgents(s snapshot.SnapshotV1) (maxAgent uint64, maxTask uint64) {
	// Agents (clients are not restored; they re-join via WS).
	w.agents = map[string]*Agent{}
	w.clients = map[string]*clientState{}
	for _, a := range s.Agents {
		aa := &Agent{
			ID:                           a.ID,
			Name:                         a.Name,
			OrgID:                        a.OrgID,
			CurrentWorldID:               a.CurrentWorldID,
			WorldSwitchCooldownUntilTick: a.WorldSwitchCooldownUntilTick,
			Pos:                          Vec3i{X: a.Pos[0], Y: a.Pos[1], Z: a.Pos[2]},
			Yaw:                          a.Yaw,
			HP:                           a.HP,
			Hunger:                       a.Hunger,
			StaminaMilli:                 a.StaminaMilli,
			RepTrade:                     a.RepTrade,
			RepBuild:                     a.RepBuild,
			RepSocial:                    a.RepSocial,
			RepLaw:                       a.RepLaw,
			Fun: FunScore{
				Novelty:    a.FunNovelty,
				Creation:   a.FunCreation,
				Social:     a.FunSocial,
				Influence:  a.FunInfluence,
				Narrative:  a.FunNarrative,
				RiskRescue: a.FunRiskRescue,
			},
			Inventory: map[string]int{},
		}
		for item, n := range a.Inventory {
			if n > 0 {
				aa.Inventory[item] = n
			}
		}
		if len(a.Memory) > 0 {
			aa.Memory = map[string]memoryEntry{}
			for k, e := range a.Memory {
				if k == "" {
					continue
				}
				if e.ExpiryTick != 0 && s.Header.Tick >= e.ExpiryTick {
					continue
				}
				aa.Memory[k] = memoryEntry{Value: e.Value, ExpiryTick: e.ExpiryTick}
			}
			if len(aa.Memory) == 0 {
				aa.Memory = nil
			}
		}
		if len(a.RateWindows) > 0 {
			aa.rl = map[string]*rateWindow{}
			for k, rw := range a.RateWindows {
				if k == "" {
					continue
				}
				if rw.Count <= 0 {
					continue
				}
				aa.rl[k] = &rateWindow{StartTick: rw.StartTick, Count: rw.Count}
			}
			if len(aa.rl) == 0 {
				aa.rl = nil
			}
		}
		if a.MoveTask != nil {
			aa.MoveTask = &tasks.MovementTask{
				TaskID:      a.MoveTask.TaskID,
				Kind:        tasks.Kind(a.MoveTask.Kind),
				Target:      tasks.Vec3i{X: a.MoveTask.Target[0], Y: a.MoveTask.Target[1], Z: a.MoveTask.Target[2]},
				Tolerance:   a.MoveTask.Tolerance,
				TargetID:    a.MoveTask.TargetID,
				Distance:    a.MoveTask.Distance,
				StartPos:    tasks.Vec3i{X: a.MoveTask.StartPos[0], Y: a.MoveTask.StartPos[1], Z: a.MoveTask.StartPos[2]},
				StartedTick: a.MoveTask.StartedTick,
			}
			if n, ok := parseUintAfterPrefix("T", a.MoveTask.TaskID); ok && n > maxTask {
				maxTask = n
			}
		}
		if a.WorkTask != nil {
			aa.WorkTask = &tasks.WorkTask{
				TaskID:       a.WorkTask.TaskID,
				Kind:         tasks.Kind(a.WorkTask.Kind),
				BlockPos:     tasks.Vec3i{X: a.WorkTask.BlockPos[0], Y: a.WorkTask.BlockPos[1], Z: a.WorkTask.BlockPos[2]},
				RecipeID:     a.WorkTask.RecipeID,
				ItemID:       a.WorkTask.ItemID,
				Count:        a.WorkTask.Count,
				BlueprintID:  a.WorkTask.BlueprintID,
				Anchor:       tasks.Vec3i{X: a.WorkTask.Anchor[0], Y: a.WorkTask.Anchor[1], Z: a.WorkTask.Anchor[2]},
				Rotation:     a.WorkTask.Rotation,
				BuildIndex:   a.WorkTask.BuildIndex,
				TargetID:     a.WorkTask.TargetID,
				SrcContainer: a.WorkTask.SrcContainer,
				DstContainer: a.WorkTask.DstContainer,
				StartedTick:  a.WorkTask.StartedTick,
				WorkTicks:    a.WorkTask.WorkTicks,
			}
			if n, ok := parseUintAfterPrefix("T", a.WorkTask.TaskID); ok && n > maxTask {
				maxTask = n
			}
		}
		aa.initDefaults()
		if aa.CurrentWorldID == "" {
			aa.CurrentWorldID = w.cfg.ID
		}
		// Restore fun-score anti-exploit and novelty memory state.
		if len(a.SeenBiomes) > 0 {
			for _, b := range a.SeenBiomes {
				if b != "" {
					aa.seenBiomes[b] = true
				}
			}
		}
		if len(a.SeenRecipes) > 0 {
			for _, r := range a.SeenRecipes {
				if r != "" {
					aa.seenRecipes[r] = true
				}
			}
		}
		if len(a.SeenEvents) > 0 {
			for _, e := range a.SeenEvents {
				if e != "" {
					aa.seenEvents[e] = true
				}
			}
		}
		if len(a.FunDecay) > 0 {
			aa.funDecay = map[string]*funDecayWindow{}
			for k, v := range a.FunDecay {
				if k == "" {
					continue
				}
				aa.funDecay[k] = &funDecayWindow{StartTick: v.StartTick, Count: v.Count}
			}
		}
		w.agents[aa.ID] = aa
		if n, ok := parseUintAfterPrefix("A", aa.ID); ok && n > maxAgent {
			maxAgent = n
		}
	}
	return maxAgent, maxTask
}

func (w *World) importSnapshotClaims(s snapshot.SnapshotV1) (maxLand uint64) {
	w.claims = map[string]*LandClaim{}
	for _, c := range s.Claims {
		claimType := c.ClaimType
		if claimType == "" {
			claimType = ClaimTypeDefault
		}
		members := map[string]bool{}
		for _, mid := range c.Members {
			if mid != "" {
				members[mid] = true
			}
		}
		w.claims[c.LandID] = &LandClaim{
			LandID:    c.LandID,
			Owner:     c.Owner,
			ClaimType: claimType,
			Anchor:    Vec3i{X: c.Anchor[0], Y: c.Anchor[1], Z: c.Anchor[2]},
			Radius:    c.Radius,
			Flags: ClaimFlags{
				AllowBuild:  c.Flags.AllowBuild,
				AllowBreak:  c.Flags.AllowBreak,
				AllowDamage: c.Flags.AllowDamage,
				AllowTrade:  c.Flags.AllowTrade,
			},
			Members:            members,
			MarketTax:          c.MarketTax,
			CurfewEnabled:      c.CurfewEnabled,
			CurfewStart:        c.CurfewStart,
			CurfewEnd:          c.CurfewEnd,
			FineBreakEnabled:   c.FineBreakEnabled,
			FineBreakItem:      c.FineBreakItem,
			FineBreakPerBlock:  c.FineBreakPerBlock,
			AccessPassEnabled:  c.AccessPassEnabled,
			AccessTicketItem:   c.AccessTicketItem,
			AccessTicketCost:   c.AccessTicketCost,
			MaintenanceDueTick: c.MaintenanceDueTick,
			MaintenanceStage:   c.MaintenanceStage,
		}
		if n, ok := parseLandNum(c.LandID); ok && n > maxLand {
			maxLand = n
		}
	}
	return maxLand
}
