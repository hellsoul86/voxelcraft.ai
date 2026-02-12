package snapshot

import (
	"sort"

	snapv1 "voxelcraft.ai/internal/persistence/snapshot"
	statspkg "voxelcraft.ai/internal/sim/world/feature/director/stats"
	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func ExportStats(s *statspkg.WorldStats) *snapv1.StatsV1 {
	if s == nil {
		return nil
	}
	out := &snapv1.StatsV1{
		BucketTicks: s.BucketTicks,
		WindowTicks: s.WindowTicksV,
		CurIdx:      s.CurIdx,
		CurBase:     s.CurBase,
		Buckets:     make([]snapv1.StatsBucketV1, len(s.Buckets)),
	}
	for i, b := range s.Buckets {
		out.Buckets[i] = snapv1.StatsBucketV1{
			Trades:             b.Trades,
			Denied:             b.Denied,
			ChunksDiscovered:   b.ChunksDiscovered,
			BlueprintsComplete: b.BlueprintsComplete,
		}
	}
	if len(s.SeenChunks) > 0 {
		keys := make([]statspkg.ChunkKey, 0, len(s.SeenChunks))
		for k := range s.SeenChunks {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].CX != keys[j].CX {
				return keys[i].CX < keys[j].CX
			}
			return keys[i].CZ < keys[j].CZ
		})
		out.SeenChunks = make([]snapv1.ChunkKeyV1, 0, len(keys))
		for _, k := range keys {
			out.SeenChunks = append(out.SeenChunks, snapv1.ChunkKeyV1{CX: k.CX, CZ: k.CZ})
		}
	}
	return out
}

func ExportTrades(trades map[string]*modelpkg.Trade) []snapv1.TradeV1 {
	tradeIDs := make([]string, 0, len(trades))
	for id := range trades {
		tradeIDs = append(tradeIDs, id)
	}
	sort.Strings(tradeIDs)
	out := make([]snapv1.TradeV1, 0, len(tradeIDs))
	for _, id := range tradeIDs {
		tr := trades[id]
		if tr == nil {
			continue
		}
		offer := map[string]int{}
		for k, v := range tr.Offer {
			if v != 0 {
				offer[k] = v
			}
		}
		req := map[string]int{}
		for k, v := range tr.Request {
			if v != 0 {
				req[k] = v
			}
		}
		out = append(out, snapv1.TradeV1{
			TradeID:     tr.TradeID,
			From:        tr.From,
			To:          tr.To,
			Offer:       offer,
			Request:     req,
			CreatedTick: tr.CreatedTick,
		})
	}
	return out
}

func ExportBoards(boards map[string]*modelpkg.Board) []snapv1.BoardV1 {
	boardIDs := make([]string, 0, len(boards))
	for id := range boards {
		boardIDs = append(boardIDs, id)
	}
	sort.Strings(boardIDs)
	out := make([]snapv1.BoardV1, 0, len(boardIDs))
	for _, id := range boardIDs {
		b := boards[id]
		if b == nil {
			continue
		}
		posts := make([]snapv1.BoardPostV1, 0, len(b.Posts))
		for _, p := range b.Posts {
			posts = append(posts, snapv1.BoardPostV1{
				PostID: p.PostID,
				Author: p.Author,
				Title:  p.Title,
				Body:   p.Body,
				Tick:   p.Tick,
			})
		}
		out = append(out, snapv1.BoardV1{BoardID: id, Posts: posts})
	}
	return out
}

func ExportAgents(nowTick uint64, agents map[string]*modelpkg.Agent) []snapv1.AgentV1 {
	sorted := make([]*modelpkg.Agent, 0, len(agents))
	for _, a := range agents {
		sorted = append(sorted, a)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	out := make([]snapv1.AgentV1, 0, len(sorted))
	for _, a := range sorted {
		if a == nil {
			continue
		}
		inv := PositiveMap(a.Inventory)

		var mem map[string]snapv1.MemoryEntryV1
		if len(a.Memory) > 0 {
			mem = map[string]snapv1.MemoryEntryV1{}
			for k, e := range a.Memory {
				if k == "" {
					continue
				}
				if e.ExpiryTick != 0 && nowTick >= e.ExpiryTick {
					continue
				}
				mem[k] = snapv1.MemoryEntryV1{Value: e.Value, ExpiryTick: e.ExpiryTick}
			}
			if len(mem) == 0 {
				mem = nil
			}
		}

		var rateWindows map[string]snapv1.RateWindowV1
		if rws := a.RateWindowsSnapshot(); len(rws) > 0 {
			rateWindows = map[string]snapv1.RateWindowV1{}
			for k, rw := range rws {
				if k == "" || rw.Count <= 0 {
					continue
				}
				rateWindows[k] = snapv1.RateWindowV1{StartTick: rw.StartTick, Count: rw.Count}
			}
			if len(rateWindows) == 0 {
				rateWindows = nil
			}
		}

		var funDecay map[string]snapv1.FunDecayV1
		if fd := a.FunDecaySnapshot(); len(fd) > 0 {
			funDecay = map[string]snapv1.FunDecayV1{}
			for k, v := range fd {
				if k == "" || v.Count <= 0 {
					continue
				}
				funDecay[k] = snapv1.FunDecayV1{StartTick: v.StartTick, Count: v.Count}
			}
			if len(funDecay) == 0 {
				funDecay = nil
			}
		}

		// Tasks.
		var moveTask *snapv1.MovementTaskV1
		if a.MoveTask != nil {
			mt := a.MoveTask
			moveTask = &snapv1.MovementTaskV1{
				TaskID:      mt.TaskID,
				Kind:        string(mt.Kind),
				Target:      [3]int{mt.Target.X, mt.Target.Y, mt.Target.Z},
				Tolerance:   mt.Tolerance,
				TargetID:    mt.TargetID,
				Distance:    mt.Distance,
				StartPos:    [3]int{mt.StartPos.X, mt.StartPos.Y, mt.StartPos.Z},
				StartedTick: mt.StartedTick,
			}
		}
		var workTask *snapv1.WorkTaskV1
		if a.WorkTask != nil {
			wt := a.WorkTask
			workTask = &snapv1.WorkTaskV1{
				TaskID:       wt.TaskID,
				Kind:         string(wt.Kind),
				BlockPos:     [3]int{wt.BlockPos.X, wt.BlockPos.Y, wt.BlockPos.Z},
				RecipeID:     wt.RecipeID,
				ItemID:       wt.ItemID,
				Count:        wt.Count,
				BlueprintID:  wt.BlueprintID,
				Anchor:       [3]int{wt.Anchor.X, wt.Anchor.Y, wt.Anchor.Z},
				Rotation:     wt.Rotation,
				BuildIndex:   wt.BuildIndex,
				TargetID:     wt.TargetID,
				SrcContainer: wt.SrcContainer,
				DstContainer: wt.DstContainer,
				StartedTick:  wt.StartedTick,
				WorkTicks:    wt.WorkTicks,
			}
		}

		out = append(out, snapv1.AgentV1{
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
			SeenBiomes:                   a.SeenBiomesSorted(),
			SeenRecipes:                  a.SeenRecipesSorted(),
			SeenEvents:                   a.SeenEventsSorted(),
			FunDecay:                     funDecay,
			MoveTask:                     moveTask,
			WorkTask:                     workTask,
		})
	}
	return out
}

func ExportClaims(claims map[string]*modelpkg.LandClaim) []snapv1.ClaimV1 {
	claimIDs := make([]string, 0, len(claims))
	for id := range claims {
		claimIDs = append(claimIDs, id)
	}
	sort.Strings(claimIDs)
	out := make([]snapv1.ClaimV1, 0, len(claimIDs))
	for _, id := range claimIDs {
		c := claims[id]
		if c == nil {
			continue
		}
		members := make([]string, 0, len(c.Members))
		for aid, ok := range c.Members {
			if aid == "" || !ok {
				continue
			}
			members = append(members, aid)
		}
		sort.Strings(members)
		out = append(out, snapv1.ClaimV1{
			LandID:    c.LandID,
			Owner:     c.Owner,
			ClaimType: c.ClaimType,
			Anchor:    c.Anchor.ToArray(),
			Radius:    c.Radius,
			Flags: snapv1.ClaimFlagsV1{
				AllowBuild:  c.Flags.AllowBuild,
				AllowBreak:  c.Flags.AllowBreak,
				AllowDamage: c.Flags.AllowDamage,
				AllowTrade:  c.Flags.AllowTrade,
			},
			Members:             members,
			MarketTax:           c.MarketTax,
			CurfewEnabled:       c.CurfewEnabled,
			CurfewStart:         c.CurfewStart,
			CurfewEnd:           c.CurfewEnd,
			FineBreakEnabled:    c.FineBreakEnabled,
			FineBreakItem:       c.FineBreakItem,
			FineBreakPerBlock:   c.FineBreakPerBlock,
			AccessPassEnabled:   c.AccessPassEnabled,
			AccessTicketItem:    c.AccessTicketItem,
			AccessTicketCost:    c.AccessTicketCost,
			MaintenanceDueTick:  c.MaintenanceDueTick,
			MaintenanceStage:    c.MaintenanceStage,
		})
	}
	return out
}

func ExportContainers(containers map[modelpkg.Vec3i]*modelpkg.Container) []snapv1.ContainerV1 {
	poses := make([]modelpkg.Vec3i, 0, len(containers))
	for p := range containers {
		poses = append(poses, p)
	}
	sort.Slice(poses, func(i, j int) bool {
		if poses[i].X != poses[j].X {
			return poses[i].X < poses[j].X
		}
		if poses[i].Y != poses[j].Y {
			return poses[i].Y < poses[j].Y
		}
		return poses[i].Z < poses[j].Z
	})
	out := make([]snapv1.ContainerV1, 0, len(poses))
	for _, pos := range poses {
		c := containers[pos]
		if c == nil || c.Type == "" {
			continue
		}
		out = append(out, snapv1.ContainerV1{
			Type:      c.Type,
			Pos:       c.Pos.ToArray(),
			Inventory: PositiveMap(c.Inventory),
			Reserved:  PositiveMap(c.Reserved),
			Owed:      PositiveNestedMap(c.Owed),
		})
	}
	return out
}

func ExportItems(nowTick uint64, items map[string]*modelpkg.ItemEntity) []snapv1.ItemEntityV1 {
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]snapv1.ItemEntityV1, 0, len(ids))
		for _, id := range ids {
			e := items[id]
			if e == nil {
				continue
			}
			if e.Item == "" || e.Count <= 0 {
				continue
			}
			// Skip expired entities at export time to keep snapshots compact and deterministic.
			if e.ExpiresTick != 0 && nowTick >= e.ExpiresTick {
				continue
			}
		out = append(out, snapv1.ItemEntityV1{
			EntityID:    e.EntityID,
			Pos:         e.Pos.ToArray(),
			Item:        e.Item,
			Count:       e.Count,
			CreatedTick: e.CreatedTick,
			ExpiresTick: e.ExpiresTick,
		})
	}
	return out
}

func ExportContracts(contracts map[string]*modelpkg.Contract) []snapv1.ContractV1 {
	ids := make([]string, 0, len(contracts))
	for id := range contracts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]snapv1.ContractV1, 0, len(ids))
	for _, id := range ids {
		c := contracts[id]
		if c == nil {
			continue
		}
		out = append(out, snapv1.ContractV1{
			ContractID:   c.ContractID,
			TerminalPos:  c.TerminalPos.ToArray(),
			Poster:       c.Poster,
			Acceptor:     c.Acceptor,
			Kind:         c.Kind,
			Requirements: PositiveMap(c.Requirements),
			Reward:       PositiveMap(c.Reward),
			Deposit:      PositiveMap(c.Deposit),
			BlueprintID:  c.BlueprintID,
			Anchor:       c.Anchor.ToArray(),
			Rotation:     c.Rotation,
			CreatedTick:  c.CreatedTick,
			DeadlineTick: c.DeadlineTick,
			State:        string(c.State),
		})
	}
	return out
}

func ExportLaws(laws map[string]*lawspkg.Law) []snapv1.LawV1 {
	ids := make([]string, 0, len(laws))
	for id := range laws {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]snapv1.LawV1, 0, len(ids))
	for _, id := range ids {
		l := laws[id]
		if l == nil {
			continue
		}
		params := map[string]string{}
		for k, v := range l.Params {
			if k == "" || v == "" {
				continue
			}
			params[k] = v
		}
		votes := map[string]string{}
		for aid, v := range l.Votes {
			if aid == "" || v == "" {
				continue
			}
			votes[aid] = v
		}
		out = append(out, snapv1.LawV1{
			LawID:         l.LawID,
			LandID:        l.LandID,
			TemplateID:    l.TemplateID,
			Title:         l.Title,
			Params:        params,
			ProposedBy:    l.ProposedBy,
			ProposedTick:  l.ProposedTick,
			NoticeEndsTick: l.NoticeEndsTick,
			VoteEndsTick:   l.VoteEndsTick,
			Status:        string(l.Status),
			Votes:         votes,
		})
	}
	return out
}

func ExportOrgs(orgs map[string]*modelpkg.Organization) []snapv1.OrgV1 {
	ids := make([]string, 0, len(orgs))
	for id := range orgs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]snapv1.OrgV1, 0, len(ids))
	for _, id := range ids {
		org := orgs[id]
		if org == nil {
			continue
		}
		members := map[string]string{}
		for aid, role := range org.Members {
			if aid == "" || role == "" {
				continue
			}
			members[aid] = string(role)
		}
		treasuryByWorld := map[string]map[string]int{}
		for wid, m := range org.TreasuryByWorld {
			if wid == "" || len(m) == 0 {
				continue
			}
			treasuryByWorld[wid] = PositiveMap(m)
		}
		out = append(out, snapv1.OrgV1{
			OrgID:           org.OrgID,
			Kind:            string(org.Kind),
			Name:            org.Name,
			CreatedTick:     org.CreatedTick,
			MetaVersion:     org.MetaVersion,
			Members:         members,
			Treasury:        PositiveMap(org.Treasury),
			TreasuryByWorld: treasuryByWorld,
		})
	}
	return out
}

func ExportStructures(structures map[string]*modelpkg.Structure) []snapv1.StructureV1 {
	ids := make([]string, 0, len(structures))
	for id := range structures {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]snapv1.StructureV1, 0, len(ids))
	for _, id := range ids {
		s := structures[id]
		if s == nil {
			continue
		}
		usedBy := map[string]uint64{}
		for aid, tick := range s.UsedBy {
			if aid == "" || tick == 0 {
				continue
			}
			usedBy[aid] = tick
		}
		out = append(out, snapv1.StructureV1{
			StructureID:   s.StructureID,
			BlueprintID:   s.BlueprintID,
			BuilderID:     s.BuilderID,
			Anchor:        s.Anchor.ToArray(),
			Rotation:      s.Rotation,
			Min:           s.Min.ToArray(),
			Max:           s.Max.ToArray(),
			CompletedTick: s.CompletedTick,
			AwardDueTick:  s.AwardDueTick,
			Awarded:       s.Awarded,
			UsedBy:        usedBy,
			LastInfluenceDay: s.LastInfluenceDay,
		})
	}
	return out
}

func ExportSigns(signs map[modelpkg.Vec3i]*modelpkg.Sign) []snapv1.SignV1 {
	poses := make([]modelpkg.Vec3i, 0, len(signs))
	for p := range signs {
		poses = append(poses, p)
	}
	sort.Slice(poses, func(i, j int) bool {
		if poses[i].X != poses[j].X {
			return poses[i].X < poses[j].X
		}
		if poses[i].Y != poses[j].Y {
			return poses[i].Y < poses[j].Y
		}
		return poses[i].Z < poses[j].Z
	})
	out := make([]snapv1.SignV1, 0, len(poses))
	for _, p := range poses {
		s := signs[p]
		if s == nil {
			continue
		}
		out = append(out, snapv1.SignV1{
			Pos:         s.Pos.ToArray(),
			Text:        s.Text,
			UpdatedTick: s.UpdatedTick,
			UpdatedBy:   s.UpdatedBy,
		})
	}
	return out
}

func ExportConveyors(conveyors map[modelpkg.Vec3i]modelpkg.ConveyorMeta) []snapv1.ConveyorV1 {
	poses := make([]modelpkg.Vec3i, 0, len(conveyors))
	for p := range conveyors {
		poses = append(poses, p)
	}
	sort.Slice(poses, func(i, j int) bool {
		if poses[i].X != poses[j].X {
			return poses[i].X < poses[j].X
		}
		if poses[i].Y != poses[j].Y {
			return poses[i].Y < poses[j].Y
		}
		return poses[i].Z < poses[j].Z
	})
	out := make([]snapv1.ConveyorV1, 0, len(poses))
	for _, p := range poses {
		m := conveyors[p]
		out = append(out, snapv1.ConveyorV1{
			Pos: p.ToArray(),
			DX:  int(m.DX),
			DZ:  int(m.DZ),
		})
	}
	return out
}

func ExportSwitches(switches map[modelpkg.Vec3i]bool) []snapv1.SwitchV1 {
	poses := make([]modelpkg.Vec3i, 0, len(switches))
	for p := range switches {
		poses = append(poses, p)
	}
	sort.Slice(poses, func(i, j int) bool {
		if poses[i].X != poses[j].X {
			return poses[i].X < poses[j].X
		}
		if poses[i].Y != poses[j].Y {
			return poses[i].Y < poses[j].Y
		}
		return poses[i].Z < poses[j].Z
	})
	out := make([]snapv1.SwitchV1, 0, len(poses))
	for _, p := range poses {
		out = append(out, snapv1.SwitchV1{Pos: p.ToArray(), On: switches[p]})
	}
	return out
}
