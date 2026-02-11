package world

import (
	"sort"

	"voxelcraft.ai/internal/persistence/snapshot"
)

func (w *World) exportSnapshotAgents(nowTick uint64) []snapshot.AgentV1 {
	agents := make([]*Agent, 0, len(w.agents))
	for _, a := range w.agents {
		agents = append(agents, a)
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].ID < agents[j].ID })
	agentSnaps := make([]snapshot.AgentV1, 0, len(agents))
	for _, a := range agents {
		inv := make(map[string]int, len(a.Inventory))
		for k, v := range a.Inventory {
			if v != 0 {
				inv[k] = v
			}
		}

		var mem map[string]snapshot.MemoryEntryV1
		if len(a.Memory) > 0 {
			mem = map[string]snapshot.MemoryEntryV1{}
			for k, e := range a.Memory {
				if k == "" {
					continue
				}
				if e.ExpiryTick != 0 && nowTick >= e.ExpiryTick {
					continue
				}
				mem[k] = snapshot.MemoryEntryV1{Value: e.Value, ExpiryTick: e.ExpiryTick}
			}
			if len(mem) == 0 {
				mem = nil
			}
		}

		var rateWindows map[string]snapshot.RateWindowV1
		if len(a.rl) > 0 {
			rateWindows = map[string]snapshot.RateWindowV1{}
			for k, rw := range a.rl {
				if k == "" || rw == nil {
					continue
				}
				if rw.Count <= 0 {
					continue
				}
				rateWindows[k] = snapshot.RateWindowV1{StartTick: rw.StartTick, Count: rw.Count}
			}
			if len(rateWindows) == 0 {
				rateWindows = nil
			}
		}

		seenBiomes := []string{}
		for b, ok := range a.seenBiomes {
			if ok && b != "" {
				seenBiomes = append(seenBiomes, b)
			}
		}
		sort.Strings(seenBiomes)
		seenRecipes := []string{}
		for r, ok := range a.seenRecipes {
			if ok && r != "" {
				seenRecipes = append(seenRecipes, r)
			}
		}
		sort.Strings(seenRecipes)
		seenEvents := []string{}
		for e, ok := range a.seenEvents {
			if ok && e != "" {
				seenEvents = append(seenEvents, e)
			}
		}
		sort.Strings(seenEvents)
		var funDecay map[string]snapshot.FunDecayV1
		if len(a.funDecay) > 0 {
			funDecay = map[string]snapshot.FunDecayV1{}
			for k, d := range a.funDecay {
				if k == "" || d == nil {
					continue
				}
				funDecay[k] = snapshot.FunDecayV1{StartTick: d.StartTick, Count: d.Count}
			}
			if len(funDecay) == 0 {
				funDecay = nil
			}
		}

		var mt *snapshot.MovementTaskV1
		if a.MoveTask != nil {
			t := a.MoveTask
			mt = &snapshot.MovementTaskV1{
				TaskID:      t.TaskID,
				Kind:        string(t.Kind),
				Target:      [3]int{t.Target.X, t.Target.Y, t.Target.Z},
				Tolerance:   t.Tolerance,
				TargetID:    t.TargetID,
				Distance:    t.Distance,
				StartPos:    [3]int{t.StartPos.X, t.StartPos.Y, t.StartPos.Z},
				StartedTick: t.StartedTick,
			}
		}
		var wt *snapshot.WorkTaskV1
		if a.WorkTask != nil {
			t := a.WorkTask
			wt = &snapshot.WorkTaskV1{
				TaskID:       t.TaskID,
				Kind:         string(t.Kind),
				BlockPos:     [3]int{t.BlockPos.X, t.BlockPos.Y, t.BlockPos.Z},
				RecipeID:     t.RecipeID,
				ItemID:       t.ItemID,
				Count:        t.Count,
				BlueprintID:  t.BlueprintID,
				Anchor:       [3]int{t.Anchor.X, t.Anchor.Y, t.Anchor.Z},
				Rotation:     t.Rotation,
				BuildIndex:   t.BuildIndex,
				TargetID:     t.TargetID,
				SrcContainer: t.SrcContainer,
				DstContainer: t.DstContainer,
				StartedTick:  t.StartedTick,
				WorkTicks:    t.WorkTicks,
			}
		}
		agentSnaps = append(agentSnaps, snapshot.AgentV1{
			ID:                           a.ID,
			Name:                         a.Name,
			OrgID:                        a.OrgID,
			CurrentWorldID:               a.CurrentWorldID,
			WorldSwitchCooldownUntilTick: a.WorldSwitchCooldownUntilTick,
			Pos:                          a.Pos.ToArray(),
			Yaw:                          a.Yaw,
			HP:                           a.HP,
			Hunger:                       a.Hunger,
			StaminaMilli:                 a.StaminaMilli,
			RepTrade:                     a.RepTrade,
			RepBuild:                     a.RepBuild,
			RepSocial:                    a.RepSocial,
			RepLaw:                       a.RepLaw,
			FunNovelty:                   a.Fun.Novelty,
			FunCreation:                  a.Fun.Creation,
			FunSocial:                    a.Fun.Social,
			FunInfluence:                 a.Fun.Influence,
			FunNarrative:                 a.Fun.Narrative,
			FunRiskRescue:                a.Fun.RiskRescue,
			Inventory:                    inv,
			Memory:                       mem,
			RateWindows:                  rateWindows,
			SeenBiomes:                   seenBiomes,
			SeenRecipes:                  seenRecipes,
			SeenEvents:                   seenEvents,
			FunDecay:                     funDecay,
			MoveTask:                     mt,
			WorkTask:                     wt,
		})
	}
	return agentSnaps
}
