package world

import (
	"sort"

	"voxelcraft.ai/internal/persistence/snapshot"
	snapshotfeaturepkg "voxelcraft.ai/internal/sim/world/feature/persistence/snapshot"
	"voxelcraft.ai/internal/sim/world/io/snapshotcodec"
)

func (w *World) exportChunkSnapshots() []snapshot.ChunkV1 {
	keys := w.chunks.LoadedChunkKeys()
	chunks := make([]snapshot.ChunkV1, 0, len(keys))
	for _, k := range keys {
		ch := w.chunks.chunks[k]
		blocks := make([]uint16, len(ch.Blocks))
		copy(blocks, ch.Blocks)
		chunks = append(chunks, snapshot.ChunkV1{
			CX:     k.CX,
			CZ:     k.CZ,
			Height: 1,
			Blocks: blocks,
		})
	}
	return chunks
}

func (w *World) exportSnapshot(nowTick uint64) snapshot.SnapshotV1 {
	// Snapshot must be called from the world loop goroutine.
	chunks := w.exportChunkSnapshots()
	agentSnaps := w.exportSnapshotAgents(nowTick)
	claimSnaps := w.exportSnapshotClaims()
	containerSnaps := w.exportSnapshotContainers()
	itemSnaps := w.exportSnapshotItems(nowTick)
	signSnaps := w.exportSnapshotSigns()
	conveyorSnaps := w.exportSnapshotConveyors()
	switchSnaps := w.exportSnapshotSwitches()
	tradeSnaps := w.exportSnapshotTrades()
	boardSnaps := w.exportSnapshotBoards()
	contractSnaps := w.exportSnapshotContracts()
	lawSnaps := w.exportSnapshotLaws()
	orgSnaps := w.exportSnapshotOrgs()
	structSnaps := w.exportSnapshotStructures()
	statsSnap := w.exportSnapshotStats()
	maintCost := w.exportSnapshotMaintenanceCost()
	starterItems := w.exportSnapshotStarterItems()

	return snapshot.SnapshotV1{
		Header: snapshot.Header{
			Version: 1,
			WorldID: w.cfg.ID,
			Tick:    nowTick,
		},
		Seed:                            w.cfg.Seed,
		TickRate:                        w.cfg.TickRateHz,
		DayTicks:                        w.cfg.DayTicks,
		SeasonLengthTicks:               w.cfg.SeasonLengthTicks,
		ObsRadius:                       w.cfg.ObsRadius,
		Height:                          w.cfg.Height,
		BoundaryR:                       w.cfg.BoundaryR,
		BiomeRegionSize:                 w.cfg.BiomeRegionSize,
		SpawnClearRadius:                w.cfg.SpawnClearRadius,
		OreClusterProbScalePermille:     w.cfg.OreClusterProbScalePermille,
		TerrainClusterProbScalePermille: w.cfg.TerrainClusterProbScalePermille,
		SprinkleStonePermille:           w.cfg.SprinkleStonePermille,
		SprinkleDirtPermille:            w.cfg.SprinkleDirtPermille,
		SprinkleLogPermille:             w.cfg.SprinkleLogPermille,
		StarterItems:                    starterItems,
		SnapshotEveryTicks:              w.cfg.SnapshotEveryTicks,
		DirectorEveryTicks:              w.cfg.DirectorEveryTicks,
		RateLimits: snapshot.RateLimitsV1{
			SayWindowTicks:        w.cfg.RateLimits.SayWindowTicks,
			SayMax:                w.cfg.RateLimits.SayMax,
			MarketSayWindowTicks:  w.cfg.RateLimits.MarketSayWindowTicks,
			MarketSayMax:          w.cfg.RateLimits.MarketSayMax,
			WhisperWindowTicks:    w.cfg.RateLimits.WhisperWindowTicks,
			WhisperMax:            w.cfg.RateLimits.WhisperMax,
			OfferTradeWindowTicks: w.cfg.RateLimits.OfferTradeWindowTicks,
			OfferTradeMax:         w.cfg.RateLimits.OfferTradeMax,
			PostBoardWindowTicks:  w.cfg.RateLimits.PostBoardWindowTicks,
			PostBoardMax:          w.cfg.RateLimits.PostBoardMax,
		},
		LawNoticeTicks:         w.cfg.LawNoticeTicks,
		LawVoteTicks:           w.cfg.LawVoteTicks,
		BlueprintAutoPullRange: w.cfg.BlueprintAutoPullRange,
		BlueprintBlocksPerTick: w.cfg.BlueprintBlocksPerTick,
		AccessPassCoreRadius:   w.cfg.AccessPassCoreRadius,
		MaintenanceCost:        maintCost,
		FunDecayWindowTicks:    w.cfg.FunDecayWindowTicks,
		FunDecayBase:           w.cfg.FunDecayBase,
		StructureSurvivalTicks: w.cfg.StructureSurvivalTicks,
		Weather:                w.weather,
		WeatherUntilTick:       w.weatherUntilTick,
		ActiveEventID:          w.activeEventID,
		ActiveEventStart:       w.activeEventStart,
		ActiveEventEnds:        w.activeEventEnds,
		ActiveEventCenter:      w.activeEventCenter.ToArray(),
		ActiveEventRadius:      w.activeEventRadius,
		Chunks:                 chunks,
		Agents:                 agentSnaps,
		Claims:                 claimSnaps,
		Containers:             containerSnaps,
		Items:                  itemSnaps,
		Signs:                  signSnaps,
		Conveyors:              conveyorSnaps,
		Switches:               switchSnaps,
		Trades:                 tradeSnaps,
		Boards:                 boardSnaps,
		Contracts:              contractSnaps,
		Laws:                   lawSnaps,
		Orgs:                   orgSnaps,
		Structures:             structSnaps,
		Stats:                  statsSnap,
		Counters: snapshot.CountersV1{
			NextAgent:    w.nextAgentNum.Load(),
			NextTask:     w.nextTaskNum.Load(),
			NextLand:     w.nextLandNum.Load(),
			NextTrade:    w.nextTradeNum.Load(),
			NextPost:     w.nextPostNum.Load(),
			NextContract: w.nextContractNum.Load(),
			NextLaw:      w.nextLawNum.Load(),
			NextOrg:      w.nextOrgNum.Load(),
			NextItem:     w.nextItemNum.Load(),
		},
	}
}

func (w *World) exportSnapshotStats() *snapshot.StatsV1 {
	if w.stats == nil {
		return nil
	}
	out := &snapshot.StatsV1{
		BucketTicks: w.stats.BucketTicks,
		WindowTicks: w.stats.WindowTicksV,
		CurIdx:      w.stats.CurIdx,
		CurBase:     w.stats.CurBase,
		Buckets:     make([]snapshot.StatsBucketV1, len(w.stats.Buckets)),
	}
	for i, b := range w.stats.Buckets {
		out.Buckets[i] = snapshot.StatsBucketV1{
			Trades:             b.Trades,
			Denied:             b.Denied,
			ChunksDiscovered:   b.ChunksDiscovered,
			BlueprintsComplete: b.BlueprintsComplete,
		}
	}
	if len(w.stats.SeenChunks) > 0 {
		keys := make([]StatsChunkKey, 0, len(w.stats.SeenChunks))
		for k := range w.stats.SeenChunks {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].CX != keys[j].CX {
				return keys[i].CX < keys[j].CX
			}
			return keys[i].CZ < keys[j].CZ
		})
		out.SeenChunks = make([]snapshot.ChunkKeyV1, 0, len(keys))
		for _, k := range keys {
			out.SeenChunks = append(out.SeenChunks, snapshot.ChunkKeyV1{CX: k.CX, CZ: k.CZ})
		}
	}
	return out
}

func (w *World) exportSnapshotMaintenanceCost() map[string]int {
	return snapshotcodec.PositiveMap(w.cfg.MaintenanceCost)
}

func (w *World) exportSnapshotStarterItems() map[string]int {
	return snapshotcodec.PositiveMap(w.cfg.StarterItems)
}

func (w *World) exportSnapshotTrades() []snapshot.TradeV1 {
	tradeIDs := make([]string, 0, len(w.trades))
	for id := range w.trades {
		tradeIDs = append(tradeIDs, id)
	}
	sort.Strings(tradeIDs)
	tradeSnaps := make([]snapshot.TradeV1, 0, len(tradeIDs))
	for _, id := range tradeIDs {
		tr := w.trades[id]
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
		tradeSnaps = append(tradeSnaps, snapshot.TradeV1{
			TradeID:     tr.TradeID,
			From:        tr.From,
			To:          tr.To,
			Offer:       offer,
			Request:     req,
			CreatedTick: tr.CreatedTick,
		})
	}
	return tradeSnaps
}

func (w *World) exportSnapshotBoards() []snapshot.BoardV1 {
	boardIDs := make([]string, 0, len(w.boards))
	for id := range w.boards {
		boardIDs = append(boardIDs, id)
	}
	sort.Strings(boardIDs)
	boardSnaps := make([]snapshot.BoardV1, 0, len(boardIDs))
	for _, id := range boardIDs {
		b := w.boards[id]
		if b == nil {
			continue
		}
		posts := make([]snapshot.BoardPostV1, 0, len(b.Posts))
		for _, p := range b.Posts {
			posts = append(posts, snapshot.BoardPostV1{
				PostID: p.PostID,
				Author: p.Author,
				Title:  p.Title,
				Body:   p.Body,
				Tick:   p.Tick,
			})
		}
		boardSnaps = append(boardSnaps, snapshot.BoardV1{BoardID: id, Posts: posts})
	}
	return boardSnaps
}

func (w *World) exportSnapshotAgents(nowTick uint64) []snapshot.AgentV1 {
	agents := make([]*Agent, 0, len(w.agents))
	for _, a := range w.agents {
		agents = append(agents, a)
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].ID < agents[j].ID })
	agentSnaps := make([]snapshot.AgentV1, 0, len(agents))
	for _, a := range agents {
		inv := snapshotfeaturepkg.PositiveMap(a.Inventory)

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

func (w *World) exportSnapshotClaims() []snapshot.ClaimV1 {
	claimIDs := make([]string, 0, len(w.claims))
	for id := range w.claims {
		claimIDs = append(claimIDs, id)
	}
	sort.Strings(claimIDs)
	claimSnaps := make([]snapshot.ClaimV1, 0, len(claimIDs))
	for _, id := range claimIDs {
		c := w.claims[id]
		members := []string{}
		for mid, ok := range c.Members {
			if ok {
				members = append(members, mid)
			}
		}
		sort.Strings(members)
		claimSnaps = append(claimSnaps, snapshot.ClaimV1{
			LandID:    c.LandID,
			Owner:     c.Owner,
			ClaimType: c.ClaimType,
			Anchor:    c.Anchor.ToArray(),
			Radius:    c.Radius,
			Flags: snapshot.ClaimFlagsV1{
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
		})
	}
	return claimSnaps
}

func (w *World) exportSnapshotContainers() []snapshot.ContainerV1 {
	contPos := make([]Vec3i, 0, len(w.containers))
	for p := range w.containers {
		contPos = append(contPos, p)
	}
	sort.Slice(contPos, func(i, j int) bool {
		if contPos[i].X != contPos[j].X {
			return contPos[i].X < contPos[j].X
		}
		if contPos[i].Y != contPos[j].Y {
			return contPos[i].Y < contPos[j].Y
		}
		return contPos[i].Z < contPos[j].Z
	})
	containerSnaps := make([]snapshot.ContainerV1, 0, len(contPos))
	for _, p := range contPos {
		c := w.containers[p]
		containerSnaps = append(containerSnaps, snapshot.ContainerV1{
			Type:      c.Type,
			Pos:       c.Pos.ToArray(),
			Inventory: snapshotfeaturepkg.PositiveMap(c.Inventory),
			Reserved:  snapshotfeaturepkg.PositiveMap(c.Reserved),
			Owed:      snapshotfeaturepkg.PositiveNestedMap(c.Owed),
		})
	}
	return containerSnaps
}

func (w *World) exportSnapshotItems(nowTick uint64) []snapshot.ItemEntityV1 {
	itemIDs := make([]string, 0, len(w.items))
	for id, e := range w.items {
		if e == nil || e.Item == "" || e.Count <= 0 {
			continue
		}
		if e.ExpiresTick != 0 && nowTick >= e.ExpiresTick {
			continue
		}
		itemIDs = append(itemIDs, id)
	}
	sort.Strings(itemIDs)
	itemSnaps := make([]snapshot.ItemEntityV1, 0, len(itemIDs))
	for _, id := range itemIDs {
		e := w.items[id]
		if e == nil || e.Item == "" || e.Count <= 0 {
			continue
		}
		if e.ExpiresTick != 0 && nowTick >= e.ExpiresTick {
			continue
		}
		itemSnaps = append(itemSnaps, snapshot.ItemEntityV1{
			EntityID:    e.EntityID,
			Pos:         e.Pos.ToArray(),
			Item:        e.Item,
			Count:       e.Count,
			CreatedTick: e.CreatedTick,
			ExpiresTick: e.ExpiresTick,
		})
	}
	return itemSnaps
}

func (w *World) exportSnapshotContracts() []snapshot.ContractV1 {
	contractIDs := make([]string, 0, len(w.contracts))
	for id := range w.contracts {
		contractIDs = append(contractIDs, id)
	}
	sort.Strings(contractIDs)
	contractSnaps := make([]snapshot.ContractV1, 0, len(contractIDs))
	for _, id := range contractIDs {
		c := w.contracts[id]
		req := map[string]int{}
		for k, v := range c.Requirements {
			if v != 0 {
				req[k] = v
			}
		}
		reward := map[string]int{}
		for k, v := range c.Reward {
			if v != 0 {
				reward[k] = v
			}
		}
		dep := map[string]int{}
		for k, v := range c.Deposit {
			if v != 0 {
				dep[k] = v
			}
		}
		contractSnaps = append(contractSnaps, snapshot.ContractV1{
			ContractID:   c.ContractID,
			TerminalPos:  c.TerminalPos.ToArray(),
			Poster:       c.Poster,
			Acceptor:     c.Acceptor,
			Kind:         c.Kind,
			State:        string(c.State),
			Requirements: req,
			Reward:       reward,
			Deposit:      dep,
			BlueprintID:  c.BlueprintID,
			Anchor:       c.Anchor.ToArray(),
			Rotation:     c.Rotation,
			CreatedTick:  c.CreatedTick,
			DeadlineTick: c.DeadlineTick,
		})
	}
	return contractSnaps
}

func (w *World) exportSnapshotLaws() []snapshot.LawV1 {
	lawIDs := make([]string, 0, len(w.laws))
	for id := range w.laws {
		lawIDs = append(lawIDs, id)
	}
	sort.Strings(lawIDs)
	lawSnaps := make([]snapshot.LawV1, 0, len(lawIDs))
	for _, id := range lawIDs {
		l := w.laws[id]
		if l == nil {
			continue
		}
		params := map[string]string{}
		for k, v := range l.Params {
			if v != "" {
				params[k] = v
			}
		}
		votes := map[string]string{}
		for k, v := range l.Votes {
			if v != "" {
				votes[k] = v
			}
		}
		lawSnaps = append(lawSnaps, snapshot.LawV1{
			LawID:          l.LawID,
			LandID:         l.LandID,
			TemplateID:     l.TemplateID,
			Title:          l.Title,
			Params:         params,
			Status:         string(l.Status),
			ProposedBy:     l.ProposedBy,
			ProposedTick:   l.ProposedTick,
			NoticeEndsTick: l.NoticeEndsTick,
			VoteEndsTick:   l.VoteEndsTick,
			Votes:          votes,
		})
	}
	return lawSnaps
}

func (w *World) exportSnapshotOrgs() []snapshot.OrgV1 {
	orgIDs := make([]string, 0, len(w.orgs))
	for id := range w.orgs {
		orgIDs = append(orgIDs, id)
	}
	sort.Strings(orgIDs)
	orgSnaps := make([]snapshot.OrgV1, 0, len(orgIDs))
	for _, id := range orgIDs {
		o := w.orgs[id]
		if o == nil {
			continue
		}
		members := map[string]string{}
		for aid, role := range o.Members {
			if aid != "" && role != "" {
				members[aid] = string(role)
			}
		}
		treasury := map[string]int{}
		for item, n := range w.orgTreasury(o) {
			if n != 0 {
				treasury[item] = n
			}
		}
		var treasuryByWorld map[string]map[string]int
		if len(o.TreasuryByWorld) > 0 {
			treasuryByWorld = map[string]map[string]int{}
			worldIDs := make([]string, 0, len(o.TreasuryByWorld))
			for wid := range o.TreasuryByWorld {
				worldIDs = append(worldIDs, wid)
			}
			sort.Strings(worldIDs)
			for _, wid := range worldIDs {
				src := o.TreasuryByWorld[wid]
				if len(src) == 0 {
					continue
				}
				dst := map[string]int{}
				for item, n := range src {
					if item == "" || n == 0 {
						continue
					}
					dst[item] = n
				}
				if len(dst) > 0 {
					treasuryByWorld[wid] = dst
				}
			}
			if len(treasuryByWorld) == 0 {
				treasuryByWorld = nil
			}
		}
		orgSnaps = append(orgSnaps, snapshot.OrgV1{
			OrgID:           o.OrgID,
			Kind:            string(o.Kind),
			Name:            o.Name,
			CreatedTick:     o.CreatedTick,
			MetaVersion:     o.MetaVersion,
			Members:         members,
			Treasury:        treasury,
			TreasuryByWorld: treasuryByWorld,
		})
	}
	return orgSnaps
}

func (w *World) exportSnapshotStructures() []snapshot.StructureV1 {
	structIDs := make([]string, 0, len(w.structures))
	for id := range w.structures {
		structIDs = append(structIDs, id)
	}
	sort.Strings(structIDs)
	structSnaps := make([]snapshot.StructureV1, 0, len(structIDs))
	for _, id := range structIDs {
		s := w.structures[id]
		if s == nil {
			continue
		}
		var usedBy map[string]uint64
		if len(s.UsedBy) > 0 {
			usedBy = map[string]uint64{}
			for aid, t := range s.UsedBy {
				if aid != "" && t != 0 {
					usedBy[aid] = t
				}
			}
			if len(usedBy) == 0 {
				usedBy = nil
			}
		}
		structSnaps = append(structSnaps, snapshot.StructureV1{
			StructureID:      s.StructureID,
			BlueprintID:      s.BlueprintID,
			BuilderID:        s.BuilderID,
			Anchor:           s.Anchor.ToArray(),
			Rotation:         s.Rotation,
			Min:              s.Min.ToArray(),
			Max:              s.Max.ToArray(),
			CompletedTick:    s.CompletedTick,
			AwardDueTick:     s.AwardDueTick,
			Awarded:          s.Awarded,
			UsedBy:           usedBy,
			LastInfluenceDay: s.LastInfluenceDay,
		})
	}
	return structSnaps
}

func (w *World) exportSnapshotSigns() []snapshot.SignV1 {
	signPos := make([]Vec3i, 0, len(w.signs))
	for p, s := range w.signs {
		if s == nil {
			continue
		}
		if w.blockName(w.chunks.GetBlock(p)) != "SIGN" {
			continue
		}
		signPos = append(signPos, p)
	}
	sort.Slice(signPos, func(i, j int) bool {
		if signPos[i].X != signPos[j].X {
			return signPos[i].X < signPos[j].X
		}
		if signPos[i].Y != signPos[j].Y {
			return signPos[i].Y < signPos[j].Y
		}
		return signPos[i].Z < signPos[j].Z
	})
	signSnaps := make([]snapshot.SignV1, 0, len(signPos))
	for _, p := range signPos {
		s := w.signs[p]
		if s == nil {
			continue
		}
		signSnaps = append(signSnaps, snapshot.SignV1{
			Pos:         p.ToArray(),
			Text:        s.Text,
			UpdatedTick: s.UpdatedTick,
			UpdatedBy:   s.UpdatedBy,
		})
	}
	return signSnaps
}

func (w *World) exportSnapshotConveyors() []snapshot.ConveyorV1 {
	conveyorPos := make([]Vec3i, 0, len(w.conveyors))
	for p := range w.conveyors {
		if w.blockName(w.chunks.GetBlock(p)) != "CONVEYOR" {
			continue
		}
		conveyorPos = append(conveyorPos, p)
	}
	sort.Slice(conveyorPos, func(i, j int) bool {
		if conveyorPos[i].X != conveyorPos[j].X {
			return conveyorPos[i].X < conveyorPos[j].X
		}
		if conveyorPos[i].Y != conveyorPos[j].Y {
			return conveyorPos[i].Y < conveyorPos[j].Y
		}
		return conveyorPos[i].Z < conveyorPos[j].Z
	})
	conveyorSnaps := make([]snapshot.ConveyorV1, 0, len(conveyorPos))
	for _, p := range conveyorPos {
		m := w.conveyors[p]
		conveyorSnaps = append(conveyorSnaps, snapshot.ConveyorV1{
			Pos: p.ToArray(),
			DX:  int(m.DX),
			DZ:  int(m.DZ),
		})
	}
	return conveyorSnaps
}

func (w *World) exportSnapshotSwitches() []snapshot.SwitchV1 {
	switchPos := make([]Vec3i, 0, len(w.switches))
	for p := range w.switches {
		if w.blockName(w.chunks.GetBlock(p)) != "SWITCH" {
			continue
		}
		switchPos = append(switchPos, p)
	}
	sort.Slice(switchPos, func(i, j int) bool {
		if switchPos[i].X != switchPos[j].X {
			return switchPos[i].X < switchPos[j].X
		}
		if switchPos[i].Y != switchPos[j].Y {
			return switchPos[i].Y < switchPos[j].Y
		}
		return switchPos[i].Z < switchPos[j].Z
	})
	switchSnaps := make([]snapshot.SwitchV1, 0, len(switchPos))
	for _, p := range switchPos {
		switchSnaps = append(switchSnaps, snapshot.SwitchV1{
			Pos: p.ToArray(),
			On:  w.switches[p],
		})
	}
	return switchSnaps
}
