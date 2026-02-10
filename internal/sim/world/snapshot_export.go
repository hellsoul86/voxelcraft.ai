package world

import (
	"sort"

	"voxelcraft.ai/internal/persistence/snapshot"
)

func (w *World) ExportSnapshot(nowTick uint64) snapshot.SnapshotV1 {
	// Snapshot must be called from the world loop goroutine.
	keys := w.chunks.LoadedChunkKeys()
	chunks := make([]snapshot.ChunkV1, 0, len(keys))
	for _, k := range keys {
		ch := w.chunks.chunks[k]
		blocks := make([]uint16, len(ch.Blocks))
		copy(blocks, ch.Blocks)
		chunks = append(chunks, snapshot.ChunkV1{
			CX:     k.CX,
			CZ:     k.CZ,
			Height: ch.Height,
			Blocks: blocks,
		})
	}

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
			for k, w := range a.funDecay {
				if k == "" || w == nil {
					continue
				}
				funDecay[k] = snapshot.FunDecayV1{StartTick: w.StartTick, Count: w.Count}
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
			ID:            a.ID,
			Name:          a.Name,
			OrgID:         a.OrgID,
			Pos:           a.Pos.ToArray(),
			Yaw:           a.Yaw,
			HP:            a.HP,
			Hunger:        a.Hunger,
			StaminaMilli:  a.StaminaMilli,
			RepTrade:      a.RepTrade,
			RepBuild:      a.RepBuild,
			RepSocial:     a.RepSocial,
			RepLaw:        a.RepLaw,
			FunNovelty:    a.Fun.Novelty,
			FunCreation:   a.Fun.Creation,
			FunSocial:     a.Fun.Social,
			FunInfluence:  a.Fun.Influence,
			FunNarrative:  a.Fun.Narrative,
			FunRiskRescue: a.Fun.RiskRescue,
			Inventory:     inv,
			Memory:        mem,
			RateWindows:   rateWindows,
			SeenBiomes:    seenBiomes,
			SeenRecipes:   seenRecipes,
			SeenEvents:    seenEvents,
			FunDecay:      funDecay,
			MoveTask:      mt,
			WorkTask:      wt,
		})
	}

	// Claims.
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
			LandID: c.LandID,
			Owner:  c.Owner,
			Anchor: c.Anchor.ToArray(),
			Radius: c.Radius,
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

	// Containers (sorted by pos).
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
		inv := map[string]int{}
		for k, v := range c.Inventory {
			if v != 0 {
				inv[k] = v
			}
		}
		res := map[string]int{}
		for k, v := range c.Reserved {
			if v != 0 {
				res[k] = v
			}
		}
		owed := map[string]map[string]int{}
		for aid, m := range c.Owed {
			m2 := map[string]int{}
			for k, v := range m {
				if v != 0 {
					m2[k] = v
				}
			}
			if len(m2) > 0 {
				owed[aid] = m2
			}
		}
		containerSnaps = append(containerSnaps, snapshot.ContainerV1{
			Type:      c.Type,
			Pos:       c.Pos.ToArray(),
			Inventory: inv,
			Reserved:  res,
			Owed:      owed,
		})
	}

	// Item entities (sorted by id).
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

	// Signs (sorted by pos).
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

	// Conveyors (sorted by pos).
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

	// Switches (sorted by pos).
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

	// Trades.
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

	// Boards.
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

	// Contracts.
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

	// Laws.
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

	// Orgs.
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
		for item, n := range o.Treasury {
			if n != 0 {
				treasury[item] = n
			}
		}
		orgSnaps = append(orgSnaps, snapshot.OrgV1{
			OrgID:       o.OrgID,
			Kind:        string(o.Kind),
			Name:        o.Name,
			CreatedTick: o.CreatedTick,
			Members:     members,
			Treasury:    treasury,
		})
	}

	// Structures (fun-score state).
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

	// Stats (director feedback state).
	var statsSnap *snapshot.StatsV1
	if w.stats != nil {
		buckets := make([]snapshot.StatsBucketV1, 0, len(w.stats.buckets))
		for _, b := range w.stats.buckets {
			buckets = append(buckets, snapshot.StatsBucketV1{
				Trades:             b.Trades,
				Denied:             b.Denied,
				ChunksDiscovered:   b.ChunksDiscovered,
				BlueprintsComplete: b.BlueprintsComplete,
			})
		}
		seen := make([]snapshot.ChunkKeyV1, 0, len(w.stats.seenChunks))
		for k := range w.stats.seenChunks {
			seen = append(seen, snapshot.ChunkKeyV1{CX: k.CX, CZ: k.CZ})
		}
		sort.Slice(seen, func(i, j int) bool {
			if seen[i].CX != seen[j].CX {
				return seen[i].CX < seen[j].CX
			}
			return seen[i].CZ < seen[j].CZ
		})
		statsSnap = &snapshot.StatsV1{
			BucketTicks: w.stats.bucketTicks,
			WindowTicks: w.stats.windowTicks,
			CurIdx:      w.stats.curIdx,
			CurBase:     w.stats.curBase,
			Buckets:     buckets,
			SeenChunks:  seen,
		}
	}

	var maintCost map[string]int
	if len(w.cfg.MaintenanceCost) > 0 {
		maintCost = map[string]int{}
		for k, v := range w.cfg.MaintenanceCost {
			if k != "" && v != 0 {
				maintCost[k] = v
			}
		}
		if len(maintCost) == 0 {
			maintCost = nil
		}
	}

	return snapshot.SnapshotV1{
		Header: snapshot.Header{
			Version: 1,
			WorldID: w.cfg.ID,
			Tick:    nowTick,
		},
		Seed:               w.cfg.Seed,
		TickRate:           w.cfg.TickRateHz,
		DayTicks:           w.cfg.DayTicks,
		SeasonLengthTicks:  w.cfg.SeasonLengthTicks,
		ObsRadius:          w.cfg.ObsRadius,
		Height:             w.cfg.Height,
		BoundaryR:          w.cfg.BoundaryR,
		SnapshotEveryTicks: w.cfg.SnapshotEveryTicks,
		DirectorEveryTicks: w.cfg.DirectorEveryTicks,
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
