package world

import (
	"fmt"
	"strings"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/tasks"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
	"voxelcraft.ai/internal/sim/world/logic/ids"
)

// ImportSnapshot replaces the current in-memory world state with the snapshot.
// It sets the world's tick to snapshotTick+1 (the next tick to simulate).
//
// This must be called only when the world is stopped or from the world loop goroutine.
func (w *World) importSnapshotV1(s snapshot.SnapshotV1) error {
	if err := w.validateSnapshotImport(s); err != nil {
		return err
	}
	w.applySnapshotConfig(s)
	w.applySnapshotRuntimeState(s)

	// Rebuild chunks.
	gen := w.chunks.gen
	gen.Seed = w.cfg.Seed
	gen.BoundaryR = w.cfg.BoundaryR
	gen.BiomeRegionSize = w.cfg.BiomeRegionSize
	gen.SpawnClearRadius = w.cfg.SpawnClearRadius
	gen.OreClusterProbScalePermille = w.cfg.OreClusterProbScalePermille
	gen.TerrainClusterProbScalePermille = w.cfg.TerrainClusterProbScalePermille
	gen.SprinkleStonePermille = w.cfg.SprinkleStonePermille
	gen.SprinkleDirtPermille = w.cfg.SprinkleDirtPermille
	gen.SprinkleLogPermille = w.cfg.SprinkleLogPermille

	if err := w.importChunkSnapshots(gen, s.Chunks); err != nil {
		return err
	}

	maxAgent, maxTask := w.importSnapshotAgents(s)
	w.nextAgentNum.Store(maxU64(maxAgent, s.Counters.NextAgent))
	w.nextTaskNum.Store(maxU64(maxTask, s.Counters.NextTask))

	maxLand := w.importSnapshotClaims(s)
	w.nextLandNum.Store(maxU64(maxLand, s.Counters.NextLand))

	w.importSnapshotContainers(s)
	maxItem := w.importSnapshotItems(s)
	w.nextItemNum.Store(maxU64(maxItem, s.Counters.NextItem))
	w.importSnapshotSigns(s)
	w.importSnapshotConveyors(s)
	w.importSnapshotSwitches(s)

	maxTrade := w.importSnapshotTrades(s)
	w.nextTradeNum.Store(maxU64(maxTrade, s.Counters.NextTrade))

	maxPost := w.importSnapshotBoards(s)
	w.nextPostNum.Store(maxU64(maxPost, s.Counters.NextPost))

	maxContract := w.importSnapshotContracts(s)
	w.nextContractNum.Store(maxU64(maxContract, s.Counters.NextContract))

	maxLaw := w.importSnapshotLaws(s)
	w.nextLawNum.Store(maxU64(maxLaw, s.Counters.NextLaw))

	maxOrg := w.importSnapshotOrgs(s)
	w.nextOrgNum.Store(maxU64(maxOrg, s.Counters.NextOrg))

	w.importSnapshotStructures(s)
	w.importSnapshotStats(s)

	// Resume on the next tick.
	w.tick.Store(s.Header.Tick + 1)
	return nil
}

func maxU64(a, b uint64) uint64 {
	return ids.MaxU64(a, b)
}

func parseUintAfterPrefix(prefix, id string) (uint64, bool) {
	return ids.ParseUintAfterPrefix(prefix, id)
}

func parseLandNum(id string) (uint64, bool) {
	return ids.ParseLandNum(id)
}

func (w *World) importChunkSnapshots(gen WorldGen, chunks []snapshot.ChunkV1) error {
	store := NewChunkStore(gen)
	for _, ch := range chunks {
		if ch.Height != 1 {
			return fmt.Errorf("snapshot chunk height mismatch: got %d want 1", ch.Height)
		}
		if len(ch.Blocks) != 16*16 {
			return fmt.Errorf("snapshot chunk blocks length mismatch: got %d want %d", len(ch.Blocks), 16*16)
		}
		k := ChunkKey{CX: ch.CX, CZ: ch.CZ}
		blocks := make([]uint16, len(ch.Blocks))
		copy(blocks, ch.Blocks)
		c := &Chunk{
			CX:     ch.CX,
			CZ:     ch.CZ,
			Blocks: blocks,
		}
		_ = c.Digest()
		store.chunks[k] = c
	}
	w.chunks = store
	return nil
}

func (w *World) importSnapshotTrades(s snapshot.SnapshotV1) (maxTrade uint64) {
	w.trades = map[string]*Trade{}
	for _, tr := range s.Trades {
		offer := map[string]int{}
		for item, n := range tr.Offer {
			if n > 0 {
				offer[item] = n
			}
		}
		req := map[string]int{}
		for item, n := range tr.Request {
			if n > 0 {
				req[item] = n
			}
		}
		w.trades[tr.TradeID] = &Trade{
			TradeID:     tr.TradeID,
			From:        tr.From,
			To:          tr.To,
			Offer:       offer,
			Request:     req,
			CreatedTick: tr.CreatedTick,
		}
		if n, ok := parseUintAfterPrefix("TR", tr.TradeID); ok && n > maxTrade {
			maxTrade = n
		}
	}
	return maxTrade
}

func (w *World) importSnapshotBoards(s snapshot.SnapshotV1) (maxPost uint64) {
	w.boards = map[string]*Board{}
	for _, b := range s.Boards {
		bb := &Board{BoardID: b.BoardID}
		for _, p := range b.Posts {
			bb.Posts = append(bb.Posts, BoardPost{
				PostID: p.PostID,
				Author: p.Author,
				Title:  p.Title,
				Body:   p.Body,
				Tick:   p.Tick,
			})
			if n, ok := parseUintAfterPrefix("P", p.PostID); ok && n > maxPost {
				maxPost = n
			}
		}
		w.boards[bb.BoardID] = bb
	}
	return maxPost
}

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

func (w *World) validateSnapshotImport(s snapshot.SnapshotV1) error {
	if s.Header.Version != 1 {
		return fmt.Errorf("unsupported snapshot version: %d", s.Header.Version)
	}
	if w.cfg.Seed != s.Seed {
		return fmt.Errorf("snapshot seed mismatch: cfg=%d snap=%d", w.cfg.Seed, s.Seed)
	}
	if w.cfg.Height != s.Height {
		return fmt.Errorf("snapshot height mismatch: cfg=%d snap=%d", w.cfg.Height, s.Height)
	}
	if s.Height != 1 {
		return fmt.Errorf("unsupported snapshot height for 2D world: height=%d", s.Height)
	}
	if w.cfg.DayTicks != s.DayTicks {
		return fmt.Errorf("snapshot day_ticks mismatch: cfg=%d snap=%d", w.cfg.DayTicks, s.DayTicks)
	}
	if w.cfg.ObsRadius != s.ObsRadius {
		return fmt.Errorf("snapshot obs_radius mismatch: cfg=%d snap=%d", w.cfg.ObsRadius, s.ObsRadius)
	}
	if w.cfg.BoundaryR != s.BoundaryR {
		return fmt.Errorf("snapshot boundary_r mismatch: cfg=%d snap=%d", w.cfg.BoundaryR, s.BoundaryR)
	}
	return nil
}

func (w *World) applySnapshotConfig(s snapshot.SnapshotV1) {
	if s.BiomeRegionSize > 0 {
		w.cfg.BiomeRegionSize = s.BiomeRegionSize
	}
	if s.SpawnClearRadius > 0 {
		w.cfg.SpawnClearRadius = s.SpawnClearRadius
	}
	if s.OreClusterProbScalePermille > 0 {
		w.cfg.OreClusterProbScalePermille = s.OreClusterProbScalePermille
	}
	if s.TerrainClusterProbScalePermille > 0 {
		w.cfg.TerrainClusterProbScalePermille = s.TerrainClusterProbScalePermille
	}
	if s.SprinkleStonePermille > 0 {
		w.cfg.SprinkleStonePermille = s.SprinkleStonePermille
	}
	if s.SprinkleDirtPermille > 0 {
		w.cfg.SprinkleDirtPermille = s.SprinkleDirtPermille
	}
	if s.SprinkleLogPermille > 0 {
		w.cfg.SprinkleLogPermille = s.SprinkleLogPermille
	}

	if s.StarterItems != nil {
		w.cfg.StarterItems = map[string]int{}
		for item, n := range s.StarterItems {
			if strings.TrimSpace(item) == "" || n <= 0 {
				continue
			}
			w.cfg.StarterItems[item] = n
		}
	}

	if s.SnapshotEveryTicks > 0 {
		w.cfg.SnapshotEveryTicks = s.SnapshotEveryTicks
	}
	if s.DirectorEveryTicks > 0 {
		w.cfg.DirectorEveryTicks = s.DirectorEveryTicks
	}
	if s.SeasonLengthTicks > 0 {
		w.cfg.SeasonLengthTicks = s.SeasonLengthTicks
	}
	if s.RateLimits.SayWindowTicks > 0 ||
		s.RateLimits.SayMax > 0 ||
		s.RateLimits.MarketSayWindowTicks > 0 ||
		s.RateLimits.MarketSayMax > 0 ||
		s.RateLimits.WhisperWindowTicks > 0 ||
		s.RateLimits.WhisperMax > 0 ||
		s.RateLimits.OfferTradeWindowTicks > 0 ||
		s.RateLimits.OfferTradeMax > 0 ||
		s.RateLimits.PostBoardWindowTicks > 0 ||
		s.RateLimits.PostBoardMax > 0 {
		w.cfg.RateLimits = RateLimitConfig{
			SayWindowTicks:        s.RateLimits.SayWindowTicks,
			SayMax:                s.RateLimits.SayMax,
			MarketSayWindowTicks:  s.RateLimits.MarketSayWindowTicks,
			MarketSayMax:          s.RateLimits.MarketSayMax,
			WhisperWindowTicks:    s.RateLimits.WhisperWindowTicks,
			WhisperMax:            s.RateLimits.WhisperMax,
			OfferTradeWindowTicks: s.RateLimits.OfferTradeWindowTicks,
			OfferTradeMax:         s.RateLimits.OfferTradeMax,
			PostBoardWindowTicks:  s.RateLimits.PostBoardWindowTicks,
			PostBoardMax:          s.RateLimits.PostBoardMax,
		}
	}
	if s.LawNoticeTicks > 0 {
		w.cfg.LawNoticeTicks = s.LawNoticeTicks
	}
	if s.LawVoteTicks > 0 {
		w.cfg.LawVoteTicks = s.LawVoteTicks
	}
	if s.BlueprintAutoPullRange > 0 {
		w.cfg.BlueprintAutoPullRange = s.BlueprintAutoPullRange
	}
	if s.BlueprintBlocksPerTick > 0 {
		w.cfg.BlueprintBlocksPerTick = s.BlueprintBlocksPerTick
	}
	if s.AccessPassCoreRadius > 0 {
		w.cfg.AccessPassCoreRadius = s.AccessPassCoreRadius
	}
	if len(s.MaintenanceCost) > 0 {
		w.cfg.MaintenanceCost = map[string]int{}
		for item, n := range s.MaintenanceCost {
			if item != "" && n > 0 {
				w.cfg.MaintenanceCost[item] = n
			}
		}
		if len(w.cfg.MaintenanceCost) == 0 {
			w.cfg.MaintenanceCost = nil
		}
	}
	if s.FunDecayWindowTicks > 0 {
		w.cfg.FunDecayWindowTicks = s.FunDecayWindowTicks
	}
	if s.FunDecayBase > 0 {
		w.cfg.FunDecayBase = s.FunDecayBase
	}
	if s.StructureSurvivalTicks > 0 {
		w.cfg.StructureSurvivalTicks = s.StructureSurvivalTicks
	}
	w.cfg.applyDefaults()
}

func (w *World) applySnapshotRuntimeState(s snapshot.SnapshotV1) {
	w.weather = s.Weather
	w.weatherUntilTick = s.WeatherUntilTick
	w.activeEventID = s.ActiveEventID
	w.activeEventStart = s.ActiveEventStart
	w.activeEventEnds = s.ActiveEventEnds
	w.activeEventCenter = Vec3i{
		X: s.ActiveEventCenter[0],
		Y: s.ActiveEventCenter[1],
		Z: s.ActiveEventCenter[2],
	}
	w.activeEventRadius = s.ActiveEventRadius
}

func (w *World) importSnapshotContracts(s snapshot.SnapshotV1) (maxContract uint64) {
	w.contracts = map[string]*Contract{}
	for _, c := range s.Contracts {
		req := map[string]int{}
		for item, n := range c.Requirements {
			if n > 0 {
				req[item] = n
			}
		}
		reward := map[string]int{}
		for item, n := range c.Reward {
			if n > 0 {
				reward[item] = n
			}
		}
		dep := map[string]int{}
		for item, n := range c.Deposit {
			if n > 0 {
				dep[item] = n
			}
		}
		cc := &Contract{
			ContractID:   c.ContractID,
			TerminalPos:  Vec3i{X: c.TerminalPos[0], Y: c.TerminalPos[1], Z: c.TerminalPos[2]},
			Poster:       c.Poster,
			Acceptor:     c.Acceptor,
			Kind:         c.Kind,
			State:        ContractState(c.State),
			Requirements: req,
			Reward:       reward,
			Deposit:      dep,
			BlueprintID:  c.BlueprintID,
			Anchor:       Vec3i{X: c.Anchor[0], Y: c.Anchor[1], Z: c.Anchor[2]},
			Rotation:     c.Rotation,
			CreatedTick:  c.CreatedTick,
			DeadlineTick: c.DeadlineTick,
		}
		w.contracts[cc.ContractID] = cc
		if n, ok := parseUintAfterPrefix("C", cc.ContractID); ok && n > maxContract {
			maxContract = n
		}
	}
	return maxContract
}

func (w *World) importSnapshotLaws(s snapshot.SnapshotV1) (maxLaw uint64) {
	w.laws = map[string]*Law{}
	for _, l := range s.Laws {
		params := map[string]string{}
		for k, v := range l.Params {
			if k != "" && v != "" {
				params[k] = v
			}
		}
		votes := map[string]string{}
		for k, v := range l.Votes {
			if k != "" && v != "" {
				votes[k] = v
			}
		}
		ll := &Law{
			LawID:          l.LawID,
			LandID:         l.LandID,
			TemplateID:     l.TemplateID,
			Title:          l.Title,
			Params:         params,
			ProposedBy:     l.ProposedBy,
			ProposedTick:   l.ProposedTick,
			NoticeEndsTick: l.NoticeEndsTick,
			VoteEndsTick:   l.VoteEndsTick,
			Status:         LawStatus(l.Status),
			Votes:          votes,
		}
		w.laws[ll.LawID] = ll
		if n, ok := parseUintAfterPrefix("LAW", ll.LawID); ok && n > maxLaw {
			maxLaw = n
		}
	}
	return maxLaw
}

func (w *World) importSnapshotOrgs(s snapshot.SnapshotV1) (maxOrg uint64) {
	w.orgs = map[string]*Organization{}
	for _, o := range s.Orgs {
		members := map[string]OrgRole{}
		for aid, role := range o.Members {
			if aid == "" || role == "" {
				continue
			}
			members[aid] = OrgRole(role)
		}
		treasury := map[string]int{}
		for item, n := range o.Treasury {
			if n != 0 {
				treasury[item] = n
			}
		}
		treasuryByWorld := map[string]map[string]int{}
		for wid, src := range o.TreasuryByWorld {
			if wid == "" || len(src) == 0 {
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
		currentWorldID := w.cfg.ID
		if currentWorldID == "" {
			currentWorldID = "GLOBAL"
		}
		currentTreasury := treasuryByWorld[currentWorldID]
		if currentTreasury == nil {
			currentTreasury = map[string]int{}
			for item, n := range treasury {
				if item == "" || n == 0 {
					continue
				}
				currentTreasury[item] = n
			}
			treasuryByWorld[currentWorldID] = currentTreasury
		}
		oo := &Organization{
			OrgID:           o.OrgID,
			Kind:            OrgKind(o.Kind),
			Name:            o.Name,
			CreatedTick:     o.CreatedTick,
			MetaVersion:     o.MetaVersion,
			Members:         members,
			Treasury:        currentTreasury,
			TreasuryByWorld: treasuryByWorld,
		}
		if oo.MetaVersion == 0 && len(oo.Members) > 0 {
			oo.MetaVersion = 1
		}
		w.orgs[oo.OrgID] = oo
		if n, ok := parseUintAfterPrefix("ORG", oo.OrgID); ok && n > maxOrg {
			maxOrg = n
		}
	}
	return maxOrg
}

func (w *World) importSnapshotStructures(s snapshot.SnapshotV1) {
	w.structures = map[string]*Structure{}
	for _, ss := range s.Structures {
		id := ss.StructureID
		if id == "" {
			continue
		}
		usedBy := map[string]uint64{}
		for aid, t := range ss.UsedBy {
			if aid != "" && t != 0 {
				usedBy[aid] = t
			}
		}
		w.structures[id] = &Structure{
			StructureID:      id,
			BlueprintID:      ss.BlueprintID,
			BuilderID:        ss.BuilderID,
			Anchor:           Vec3i{X: ss.Anchor[0], Y: ss.Anchor[1], Z: ss.Anchor[2]},
			Rotation:         blueprint.NormalizeRotation(ss.Rotation),
			Min:              Vec3i{X: ss.Min[0], Y: ss.Min[1], Z: ss.Min[2]},
			Max:              Vec3i{X: ss.Max[0], Y: ss.Max[1], Z: ss.Max[2]},
			CompletedTick:    ss.CompletedTick,
			AwardDueTick:     ss.AwardDueTick,
			Awarded:          ss.Awarded,
			UsedBy:           usedBy,
			LastInfluenceDay: ss.LastInfluenceDay,
		}
	}
}

func (w *World) importSnapshotStats(s snapshot.SnapshotV1) {
	if s.Stats != nil && len(s.Stats.Buckets) > 0 && s.Stats.BucketTicks > 0 {
		st := &WorldStats{
			BucketTicks:  s.Stats.BucketTicks,
			WindowTicksV: s.Stats.WindowTicks,
			Buckets:      make([]StatsBucket, len(s.Stats.Buckets)),
			CurIdx:       s.Stats.CurIdx,
			CurBase:      s.Stats.CurBase,
			SeenChunks:   map[StatsChunkKey]bool{},
		}
		for i, b := range s.Stats.Buckets {
			st.Buckets[i] = StatsBucket{
				Trades:             b.Trades,
				Denied:             b.Denied,
				ChunksDiscovered:   b.ChunksDiscovered,
				BlueprintsComplete: b.BlueprintsComplete,
			}
		}
		if st.CurIdx < 0 || st.CurIdx >= len(st.Buckets) {
			st.CurIdx = 0
		}
		for _, k := range s.Stats.SeenChunks {
			st.SeenChunks[StatsChunkKey{CX: k.CX, CZ: k.CZ}] = true
		}
		w.stats = st
		return
	}
	w.stats = NewWorldStats(300, 72000)
}

func (w *World) importSnapshotContainers(s snapshot.SnapshotV1) {
	w.containers = map[Vec3i]*Container{}
	for _, c := range s.Containers {
		pos := Vec3i{X: c.Pos[0], Y: c.Pos[1], Z: c.Pos[2]}
		cc := &Container{
			Type:      c.Type,
			Pos:       pos,
			Inventory: map[string]int{},
		}
		for item, n := range c.Inventory {
			if n > 0 {
				cc.Inventory[item] = n
			}
		}
		if len(c.Reserved) > 0 {
			cc.Reserved = map[string]int{}
			for item, n := range c.Reserved {
				if n > 0 {
					cc.Reserved[item] = n
				}
			}
		}
		if len(c.Owed) > 0 {
			cc.Owed = map[string]map[string]int{}
			for aid, m := range c.Owed {
				if aid == "" || len(m) == 0 {
					continue
				}
				m2 := map[string]int{}
				for item, n := range m {
					if n > 0 {
						m2[item] = n
					}
				}
				if len(m2) > 0 {
					cc.Owed[aid] = m2
				}
			}
		}
		w.containers[pos] = cc
	}
}

func (w *World) importSnapshotItems(s snapshot.SnapshotV1) (maxItem uint64) {
	w.items = map[string]*ItemEntity{}
	w.itemsAt = map[Vec3i][]string{}
	for _, it := range s.Items {
		if it.EntityID == "" || it.Item == "" || it.Count <= 0 {
			continue
		}
		if it.ExpiresTick != 0 && s.Header.Tick >= it.ExpiresTick {
			continue
		}
		pos := Vec3i{X: it.Pos[0], Y: it.Pos[1], Z: it.Pos[2]}
		e := &ItemEntity{
			EntityID:    it.EntityID,
			Pos:         pos,
			Item:        it.Item,
			Count:       it.Count,
			CreatedTick: it.CreatedTick,
			ExpiresTick: it.ExpiresTick,
		}
		w.items[e.EntityID] = e
		w.itemsAt[pos] = append(w.itemsAt[pos], e.EntityID)
		if n, ok := parseUintAfterPrefix("IT", e.EntityID); ok && n > maxItem {
			maxItem = n
		}
	}
	return maxItem
}

func (w *World) importSnapshotSigns(s snapshot.SnapshotV1) {
	w.signs = map[Vec3i]*Sign{}
	for _, ss := range s.Signs {
		pos := Vec3i{X: ss.Pos[0], Y: ss.Pos[1], Z: ss.Pos[2]}
		if w.blockName(w.chunks.GetBlock(pos)) != "SIGN" {
			continue
		}
		w.signs[pos] = &Sign{
			Pos:         pos,
			Text:        ss.Text,
			UpdatedTick: ss.UpdatedTick,
			UpdatedBy:   ss.UpdatedBy,
		}
	}
}

func (w *World) importSnapshotConveyors(s snapshot.SnapshotV1) {
	w.conveyors = map[Vec3i]ConveyorMeta{}
	for _, cv := range s.Conveyors {
		pos := Vec3i{X: cv.Pos[0], Y: cv.Pos[1], Z: cv.Pos[2]}
		if w.blockName(w.chunks.GetBlock(pos)) != "CONVEYOR" {
			continue
		}
		w.conveyors[pos] = ConveyorMeta{DX: int8(cv.DX), DZ: int8(cv.DZ)}
	}
}

func (w *World) importSnapshotSwitches(s snapshot.SnapshotV1) {
	w.switches = map[Vec3i]bool{}
	for _, sv := range s.Switches {
		pos := Vec3i{X: sv.Pos[0], Y: sv.Pos[1], Z: sv.Pos[2]}
		if w.blockName(w.chunks.GetBlock(pos)) != "SWITCH" {
			continue
		}
		w.switches[pos] = sv.On
	}
}
