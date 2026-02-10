package world

import (
	"fmt"
	"strconv"
	"strings"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/tasks"
)

// ImportSnapshot replaces the current in-memory world state with the snapshot.
// It sets the world's tick to snapshotTick+1 (the next tick to simulate).
//
// This must be called only when the world is stopped or from the world loop goroutine.
func (w *World) ImportSnapshot(s snapshot.SnapshotV1) error {
	if s.Header.Version != 1 {
		return fmt.Errorf("unsupported snapshot version: %d", s.Header.Version)
	}

	// Basic parameter consistency checks.
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
	// season_length_ticks is operational; the snapshot value is authoritative when present.
	if w.cfg.ObsRadius != s.ObsRadius {
		return fmt.Errorf("snapshot obs_radius mismatch: cfg=%d snap=%d", w.cfg.ObsRadius, s.ObsRadius)
	}
	if w.cfg.BoundaryR != s.BoundaryR {
		return fmt.Errorf("snapshot boundary_r mismatch: cfg=%d snap=%d", w.cfg.BoundaryR, s.BoundaryR)
	}

	// Operational parameters: snapshot is authoritative when present.
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

	// Reset dynamic fields.
	w.weather = s.Weather
	w.weatherUntilTick = s.WeatherUntilTick
	w.activeEventID = s.ActiveEventID
	w.activeEventStart = s.ActiveEventStart
	w.activeEventEnds = s.ActiveEventEnds
	w.activeEventCenter = Vec3i{X: s.ActiveEventCenter[0], Y: s.ActiveEventCenter[1], Z: s.ActiveEventCenter[2]}
	w.activeEventRadius = s.ActiveEventRadius

	// Rebuild chunks.
	store := NewChunkStore(w.chunks.gen)
	for _, ch := range s.Chunks {
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
			dirty:  true,
		}
		_ = c.Digest()
		store.chunks[k] = c
	}
	w.chunks = store

	// Agents (clients are not restored; they re-join via WS).
	w.agents = map[string]*Agent{}
	w.clients = map[string]*clientState{}
	var maxAgent uint64
	var maxTask uint64
	for _, a := range s.Agents {
		aa := &Agent{
			ID:           a.ID,
			Name:         a.Name,
			OrgID:        a.OrgID,
			Pos:          Vec3i{X: a.Pos[0], Y: a.Pos[1], Z: a.Pos[2]},
			Yaw:          a.Yaw,
			HP:           a.HP,
			Hunger:       a.Hunger,
			StaminaMilli: a.StaminaMilli,
			RepTrade:     a.RepTrade,
			RepBuild:     a.RepBuild,
			RepSocial:    a.RepSocial,
			RepLaw:       a.RepLaw,
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
	w.nextAgentNum.Store(maxU64(maxAgent, s.Counters.NextAgent))
	w.nextTaskNum.Store(maxU64(maxTask, s.Counters.NextTask))

	// Claims.
	w.claims = map[string]*LandClaim{}
	var maxLand uint64
	for _, c := range s.Claims {
		members := map[string]bool{}
		for _, mid := range c.Members {
			if mid != "" {
				members[mid] = true
			}
		}
		w.claims[c.LandID] = &LandClaim{
			LandID: c.LandID,
			Owner:  c.Owner,
			Anchor: Vec3i{X: c.Anchor[0], Y: c.Anchor[1], Z: c.Anchor[2]},
			Radius: c.Radius,
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
	w.nextLandNum.Store(maxU64(maxLand, s.Counters.NextLand))

	// Containers.
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

	// Item entities.
	w.items = map[string]*ItemEntity{}
	w.itemsAt = map[Vec3i][]string{}
	var maxItem uint64
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
	w.nextItemNum.Store(maxU64(maxItem, s.Counters.NextItem))

	// Signs.
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

	// Conveyors.
	w.conveyors = map[Vec3i]ConveyorMeta{}
	for _, cv := range s.Conveyors {
		pos := Vec3i{X: cv.Pos[0], Y: cv.Pos[1], Z: cv.Pos[2]}
		if w.blockName(w.chunks.GetBlock(pos)) != "CONVEYOR" {
			continue
		}
		w.conveyors[pos] = ConveyorMeta{DX: int8(cv.DX), DZ: int8(cv.DZ)}
	}

	// Switches.
	w.switches = map[Vec3i]bool{}
	for _, sv := range s.Switches {
		pos := Vec3i{X: sv.Pos[0], Y: sv.Pos[1], Z: sv.Pos[2]}
		if w.blockName(w.chunks.GetBlock(pos)) != "SWITCH" {
			continue
		}
		w.switches[pos] = sv.On
	}

	// Trades.
	w.trades = map[string]*Trade{}
	var maxTrade uint64
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
	w.nextTradeNum.Store(maxU64(maxTrade, s.Counters.NextTrade))

	// Boards + post counter.
	w.boards = map[string]*Board{}
	var maxPost uint64
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
	w.nextPostNum.Store(maxU64(maxPost, s.Counters.NextPost))

	// Contracts.
	w.contracts = map[string]*Contract{}
	var maxContract uint64
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
	w.nextContractNum.Store(maxU64(maxContract, s.Counters.NextContract))

	// Laws.
	w.laws = map[string]*Law{}
	var maxLaw uint64
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
	w.nextLawNum.Store(maxU64(maxLaw, s.Counters.NextLaw))

	// Orgs.
	w.orgs = map[string]*Organization{}
	var maxOrg uint64
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
		oo := &Organization{
			OrgID:       o.OrgID,
			Kind:        OrgKind(o.Kind),
			Name:        o.Name,
			CreatedTick: o.CreatedTick,
			Members:     members,
			Treasury:    treasury,
		}
		w.orgs[oo.OrgID] = oo
		if n, ok := parseUintAfterPrefix("ORG", oo.OrgID); ok && n > maxOrg {
			maxOrg = n
		}
	}
	w.nextOrgNum.Store(maxU64(maxOrg, s.Counters.NextOrg))

	// Structures (fun-score state).
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
			Rotation:         normalizeRotation(ss.Rotation),
			Min:              Vec3i{X: ss.Min[0], Y: ss.Min[1], Z: ss.Min[2]},
			Max:              Vec3i{X: ss.Max[0], Y: ss.Max[1], Z: ss.Max[2]},
			CompletedTick:    ss.CompletedTick,
			AwardDueTick:     ss.AwardDueTick,
			Awarded:          ss.Awarded,
			UsedBy:           usedBy,
			LastInfluenceDay: ss.LastInfluenceDay,
		}
	}

	// Director stats.
	if s.Stats != nil && len(s.Stats.Buckets) > 0 && s.Stats.BucketTicks > 0 {
		st := &WorldStats{
			bucketTicks: s.Stats.BucketTicks,
			windowTicks: s.Stats.WindowTicks,
			buckets:     make([]StatsBucket, len(s.Stats.Buckets)),
			curIdx:      s.Stats.CurIdx,
			curBase:     s.Stats.CurBase,
			seenChunks:  map[ChunkKey]bool{},
		}
		for i, b := range s.Stats.Buckets {
			st.buckets[i] = StatsBucket{
				Trades:             b.Trades,
				Denied:             b.Denied,
				ChunksDiscovered:   b.ChunksDiscovered,
				BlueprintsComplete: b.BlueprintsComplete,
			}
		}
		if st.curIdx < 0 || st.curIdx >= len(st.buckets) {
			st.curIdx = 0
		}
		for _, k := range s.Stats.SeenChunks {
			st.seenChunks[ChunkKey{CX: k.CX, CZ: k.CZ}] = true
		}
		w.stats = st
	} else {
		w.stats = NewWorldStats(300, 72000)
	}

	// Resume on the next tick.
	w.tick.Store(s.Header.Tick + 1)

	return nil
}

func maxU64(a, b uint64) uint64 {
	if a >= b {
		return a
	}
	return b
}

func parseUintAfterPrefix(prefix, id string) (uint64, bool) {
	if !strings.HasPrefix(id, prefix) {
		return 0, false
	}
	n, err := strconv.ParseUint(id[len(prefix):], 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseLandNum(id string) (uint64, bool) {
	i := strings.LastIndexByte(id, '_')
	if i < 0 || i+1 >= len(id) {
		return 0, false
	}
	n, err := strconv.ParseUint(id[i+1:], 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
