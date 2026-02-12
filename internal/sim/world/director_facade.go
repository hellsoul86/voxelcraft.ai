package world

import (
	"math"
	"sort"
	"strconv"
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	eventspkg "voxelcraft.ai/internal/sim/world/feature/director/events"
	feedbackpkg "voxelcraft.ai/internal/sim/world/feature/director/feedback"
	metricspkg "voxelcraft.ai/internal/sim/world/feature/director/metrics"
	runtimepkg "voxelcraft.ai/internal/sim/world/feature/director/runtime"
	spawnspkg "voxelcraft.ai/internal/sim/world/feature/director/spawns"
	respawnpkg "voxelcraft.ai/internal/sim/world/feature/survival/respawn"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
	"voxelcraft.ai/internal/sim/world/logic/directorcenter"
	"voxelcraft.ai/internal/sim/world/logic/mathx"
)

type directorMetrics struct {
	Trade       float64 // 0..1
	Conflict    float64 // 0..1
	Exploration float64 // 0..1
	Inequality  float64 // 0..1 (Gini)
	PublicInfra float64 // 0..1
}

func (w *World) systemDirector(nowTick uint64) {
	next, _ := runtimepkg.Expire(runtimepkg.State{
		ActiveEventID:   w.activeEventID,
		ActiveEventEnds: w.activeEventEnds,
		Weather:         w.weather,
		WeatherUntil:    w.weatherUntilTick,
	}, nowTick)
	if next.ActiveEventID == "" && w.activeEventID != "" {
		w.activeEventStart = 0
		w.activeEventCenter = Vec3i{}
		w.activeEventRadius = 0
	}
	w.activeEventID = next.ActiveEventID
	w.activeEventEnds = next.ActiveEventEnds
	w.weather = next.Weather
	w.weatherUntilTick = next.WeatherUntil

	// If an event is still active, don't schedule a new one.
	if w.activeEventID != "" {
		return
	}

	// First-week scripted cadence at the start of each in-game day.
	if w.cfg.DayTicks > 0 && nowTick%uint64(w.cfg.DayTicks) == 0 {
		dayInSeason := w.seasonDay(nowTick)
		if ev := runtimepkg.ScriptedEvent(dayInSeason); ev != "" {
			w.startEvent(nowTick, ev)
			return
		}
	}

	// After week 1, evaluate every N ticks (default 3000 ~= 10 minutes at 5Hz).
	every := uint64(w.cfg.DirectorEveryTicks)
	if !runtimepkg.ShouldEvaluate(nowTick, every) {
		return
	}

	m := w.computeDirectorMetrics(nowTick)
	weights := w.baseEventWeights()

	feedbackpkg.ApplyFeedback(weights, feedbackpkg.Metrics{
		Trade:       m.Trade,
		Conflict:    m.Conflict,
		Exploration: m.Exploration,
		Inequality:  m.Inequality,
	})

	// Sample deterministically using world seed + tick.
	ev := directorcenter.SampleWeighted(weights, hash2(w.cfg.Seed, int(nowTick), 1337))
	if ev == "" {
		return
	}
	w.startEvent(nowTick, ev)
}

func (w *World) baseEventWeights() map[string]float64 {
	weights := map[string]float64{}
	ids := make([]string, 0, len(w.catalogs.Events.ByID))
	for id := range w.catalogs.Events.ByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		t := w.catalogs.Events.ByID[id]
		if t.BaseWeight <= 0 {
			continue
		}
		weights[id] = t.BaseWeight
	}
	// Avoid immediate repeats.
	if w.activeEventID != "" {
		weights[w.activeEventID] = 0
	}
	return weights
}

func (w *World) computeDirectorMetrics(nowTick uint64) directorMetrics {
	agents := len(w.agents)
	if agents <= 0 {
		return directorMetrics{}
	}

	sum := StatsBucket{}
	windowTicks := uint64(0)
	if w.stats != nil {
		sum = w.stats.Summarize(nowTick)
		windowTicks = w.stats.WindowTicks()
	}
	if windowTicks == 0 {
		windowTicks = 72000
	}

	wealth := make([]float64, 0, len(w.agents))
	for _, a := range w.sortedAgents() {
		if a == nil {
			continue
		}
		wealth = append(wealth, metricspkg.InventoryValue(a.Inventory))
	}
	m := metricspkg.ComputeMetrics(metricspkg.EvalInput{
		Agents:      agents,
		WindowTicks: windowTicks,
		Trades:      sum.Trades,
		Denied:      sum.Denied,
		Chunks:      sum.ChunksDiscovered,
		Blueprints:  sum.BlueprintsComplete,
		Wealth:      wealth,
	})

	return directorMetrics{
		Trade:       m.Trade,
		Conflict:    m.Conflict,
		Exploration: m.Exploration,
		Inequality:  m.Inequality,
		PublicInfra: m.PublicInfra,
	}
}

func (w *World) startEvent(nowTick uint64, eventID string) {
	tpl, ok := w.catalogs.Events.ByID[eventID]
	if !ok {
		return
	}
	duration := uint64(w.cfg.DayTicks)
	if duration == 0 {
		duration = 6000
	}
	if v, ok := tpl.Params["duration_ticks"]; ok {
		if f, ok := v.(float64); ok && f > 0 {
			duration = uint64(f)
		}
	}

	w.activeEventID = eventID
	w.activeEventStart = nowTick
	w.activeEventEnds = nowTick + duration

	// Instantiate event effects (e.g. spawn a resource node).
	w.instantiateEvent(nowTick, eventID)

	// Optional weather overrides.
	switch eventID {
	case "STORM_FRONT":
		w.weather = "STORM"
		w.weatherUntilTick = w.activeEventEnds
	case "COLD_SNAP":
		w.weather = "COLD"
		w.weatherUntilTick = w.activeEventEnds
	}

	for _, a := range w.agents {
		ev := protocol.Event{
			"t":         nowTick,
			"type":      "WORLD_EVENT",
			"event_id":  eventID,
			"title":     tpl.Title,
			"summary":   tpl.Description,
			"ends_tick": w.activeEventEnds,
		}
		if w.activeEventRadius > 0 {
			ev["center"] = w.activeEventCenter.ToArray()
			ev["radius"] = w.activeEventRadius
		}
		a.AddEvent(ev)
	}
}

func (w *World) enqueueActiveEventForAgent(nowTick uint64, a *Agent) {
	if a == nil || w.activeEventID == "" || w.activeEventEnds == 0 || nowTick >= w.activeEventEnds {
		return
	}
	tpl, ok := w.catalogs.Events.ByID[w.activeEventID]
	if !ok {
		return
	}
	ev := protocol.Event{
		"t":         nowTick,
		"type":      "WORLD_EVENT",
		"event_id":  w.activeEventID,
		"title":     tpl.Title,
		"summary":   tpl.Description,
		"ends_tick": w.activeEventEnds,
	}
	if w.activeEventRadius > 0 {
		ev["center"] = w.activeEventCenter.ToArray()
		ev["radius"] = w.activeEventRadius
	}
	a.AddEvent(ev)
}

func (w *World) instantiateEvent(nowTick uint64, eventID string) {
	// Default: no location.
	w.activeEventCenter = Vec3i{}
	w.activeEventRadius = 0

	switch eventID {
	case "CRYSTAL_RIFT":
		center := w.pickEventCenter(nowTick, eventID)
		w.activeEventCenter = center
		w.activeEventRadius = 32
		w.spawnCrystalRift(nowTick, center)

	case "DEEP_VEIN":
		center := w.pickEventCenter(nowTick, eventID)
		w.activeEventCenter = center
		w.activeEventRadius = 40
		w.spawnDeepVein(nowTick, center)

	case "RUINS_GATE":
		center := w.pickEventCenter(nowTick, eventID)
		w.activeEventCenter = center
		w.activeEventRadius = 24
		w.spawnRuinsGate(nowTick, center)

	case "MARKET_WEEK":
		center := w.pickEventCenter(nowTick, eventID)
		w.activeEventCenter = center
		w.activeEventRadius = 32
		w.spawnEventNoticeBoard(nowTick, center, eventID, "市集周", "市场税临时减免，鼓励交易与签约。")

	case "BLUEPRINT_FAIR":
		center := w.pickEventCenter(nowTick, eventID)
		w.activeEventCenter = center
		w.activeEventRadius = 32
		w.spawnEventNoticeBoard(nowTick, center, eventID, "蓝图开放日", "分享与复用蓝图将获得额外影响力。")

	case "BUILDER_EXPO":
		center := w.pickEventCenter(nowTick, eventID)
		w.activeEventCenter = center
		w.activeEventRadius = 40
		theme := "MONUMENT"
		if tpl, ok := w.catalogs.Events.ByID[eventID]; ok {
			if v, ok := tpl.Params["theme"]; ok {
				if s, ok := v.(string); ok && s != "" {
					theme = s
				}
			}
		}
		w.spawnEventNoticeBoard(nowTick, center, eventID, "建筑大赛", "主题: "+theme+"。完成蓝图建造并展示。")

	case "FLOOD_WARNING":
		center := w.pickEventCenter(nowTick, eventID)
		w.activeEventCenter = center
		w.activeEventRadius = 40
		w.spawnFloodWarning(nowTick, center)
		w.spawnEventNoticeBoard(nowTick, center, eventID, "洪水风险", "低地可能被淹，建议修堤坝与迁移仓库。")

	case "BANDIT_CAMP":
		center := w.pickEventCenter(nowTick, eventID)
		w.activeEventCenter = center
		w.activeEventRadius = 24
		w.spawnBanditCamp(nowTick, center)

	case "BLIGHT_ZONE":
		center := w.pickEventCenter(nowTick, eventID)
		w.activeEventCenter = center
		w.activeEventRadius = 32
		w.spawnBlightZone(nowTick, center)
		w.spawnEventNoticeBoard(nowTick, center, eventID, "污染扩散", "在污染区行动会降低体力恢复并加速饥饿。")

	case "CIVIC_VOTE":
		center := w.pickEventCenter(nowTick, eventID)
		w.activeEventCenter = center
		w.activeEventRadius = 32
		w.spawnEventNoticeBoard(nowTick, center, eventID, "城邦选举/公投", "提出法律并投票将获得额外叙事分。")
	}
}

func (w *World) pickEventCenter(nowTick uint64, eventID string) Vec3i {
	p := directorcenter.PickEventCenter(
		w.cfg.Seed,
		w.cfg.BoundaryR,
		nowTick,
		eventID,
		func(dp directorcenter.Pos) bool {
			return w.landAt(Vec3i{X: dp.X, Y: dp.Y, Z: dp.Z}) != nil
		},
	)
	return Vec3i{X: p.X, Y: p.Y, Z: p.Z}
}

func distXZ(a, b Vec3i) int {
	return mathx.AbsInt(a.X-b.X) + mathx.AbsInt(a.Z-b.Z)
}

func (w *World) onMinedBlockDuringEvent(a *Agent, pos Vec3i, blockName string, nowTick uint64) {
	if a == nil || blockName == "" || w.activeEventID == "" || w.activeEventRadius <= 0 {
		return
	}
	if distXZ(pos, w.activeEventCenter) > w.activeEventRadius {
		return
	}
	out := eventspkg.MineOutcome(w.activeEventID, blockName)
	if !out.OK {
		return
	}
	if out.GrantItem != "" && out.GrantCount > 0 {
		a.Inventory[out.GrantItem] += out.GrantCount
	}
	w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
	if out.Narrative > 0 {
		w.addFun(a, nowTick, "NARRATIVE", "event_mine", a.FunDecayDelta("narrative:event_mine:"+w.activeEventID, out.Narrative, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	}
	if out.GoalKind != "" {
		a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": out.GoalKind})
	}
}

func (w *World) onContainerOpenedDuringEvent(a *Agent, c *Container, nowTick uint64) {
	if a == nil || c == nil || w.activeEventID == "" || w.activeEventRadius <= 0 {
		return
	}
	if distXZ(c.Pos, w.activeEventCenter) > w.activeEventRadius {
		return
	}
	out := eventspkg.OpenContainerOutcome(w.activeEventID, c.Type)
	if !out.OK {
		return
	}
	w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
	if out.Risk > 0 {
		w.addFun(a, nowTick, "RISK_RESCUE", "bandit_loot", a.FunDecayDelta("risk:bandit_loot", out.Risk, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	}
	if out.Narrative > 0 {
		w.addFun(a, nowTick, "NARRATIVE", "bandit_loot", a.FunDecayDelta("narrative:bandit_loot", out.Narrative, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	}
	if out.GoalKind != "" {
		a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": out.GoalKind})
	}
}

func (w *World) spawnCrystalRift(nowTick uint64, center Vec3i) {
	ore, ok := w.catalogs.Blocks.Index["CRYSTAL_ORE"]
	if !ok {
		return
	}
	// 2D world: spawn a compact surface cluster on y=0.
	for _, pp := range spawnspkg.Square(spawnspkg.Pos{X: center.X, Y: 0, Z: center.Z}, 2) {
		p := Vec3i{X: pp.X, Y: pp.Y, Z: pp.Z}
		from := w.chunks.GetBlock(p)
		w.chunks.SetBlock(p, ore)
		w.auditSetBlock(nowTick, "WORLD", p, from, ore, "EVENT:CRYSTAL_RIFT")
	}
}

func (w *World) spawnDeepVein(nowTick uint64, center Vec3i) {
	iron, ok1 := w.catalogs.Blocks.Index["IRON_ORE"]
	copper, ok2 := w.catalogs.Blocks.Index["COPPER_ORE"]
	if !ok1 || !ok2 {
		return
	}
	// 2D world: spawn a mixed ore patch on y=0.
	for _, pp := range spawnspkg.Square(spawnspkg.Pos{X: center.X, Y: 0, Z: center.Z}, 3) {
		p := Vec3i{X: pp.X, Y: pp.Y, Z: pp.Z}
		to := iron
		if spawnspkg.DeepVeinIsCopper(pp.X-center.X, pp.Z-center.Z) {
			to = copper
		}
		from := w.chunks.GetBlock(p)
		w.chunks.SetBlock(p, to)
		w.auditSetBlock(nowTick, "WORLD", p, from, to, "EVENT:DEEP_VEIN")
	}
}

func (w *World) spawnRuinsGate(nowTick uint64, center Vec3i) {
	brick, okB := w.catalogs.Blocks.Index["BRICK"]
	chest, okC := w.catalogs.Blocks.Index["CHEST"]
	if !okB || !okC {
		return
	}

	// Build a small ring with a loot chest in the center.
	p0 := Vec3i{X: center.X, Y: 0, Z: center.Z}

	for _, pp := range spawnspkg.Square(spawnspkg.Pos{X: p0.X, Y: p0.Y, Z: p0.Z}, 1) {
		p := Vec3i{X: pp.X, Y: pp.Y, Z: pp.Z}
		from := w.chunks.GetBlock(p)
		to := brick
		if pp.X == p0.X && pp.Z == p0.Z {
			to = chest
		}
		w.chunks.SetBlock(p, to)
		w.auditSetBlock(nowTick, "WORLD", p, from, to, "EVENT:RUINS_GATE")
		if pp.X == p0.X && pp.Z == p0.Z {
			c := w.ensureContainer(p, "CHEST")
			c.Inventory["CRYSTAL_SHARD"] += 2
			c.Inventory["IRON_INGOT"] += 4
			c.Inventory["COPPER_INGOT"] += 4
		}
	}

	// Use the chest position as the event center marker.
	w.activeEventCenter = p0
}

func (w *World) spawnEventNoticeBoard(nowTick uint64, center Vec3i, eventID string, headline string, body string) {
	board, okB := w.catalogs.Blocks.Index["BULLETIN_BOARD"]
	sign, okS := w.catalogs.Blocks.Index["SIGN"]
	if !okB || !okS {
		return
	}

	boardPos := Vec3i{X: center.X, Y: 0, Z: center.Z}
	signPos := Vec3i{X: center.X + 1, Y: 0, Z: center.Z}

	from := w.chunks.GetBlock(boardPos)
	w.chunks.SetBlock(boardPos, board)
	w.auditSetBlock(nowTick, "WORLD", boardPos, from, board, "EVENT:"+eventID)
	w.ensureBoard(boardPos)
	if b := w.boards[boardIDAt(boardPos)]; b != nil {
		postID := w.newPostID()
		b.Posts = append(b.Posts, BoardPost{
			PostID: postID,
			Author: "WORLD",
			Title:  headline,
			Body:   body,
			Tick:   nowTick,
		})
	}

	from2 := w.chunks.GetBlock(signPos)
	w.chunks.SetBlock(signPos, sign)
	w.auditSetBlock(nowTick, "WORLD", signPos, from2, sign, "EVENT:"+eventID)
	s := w.ensureSign(signPos)
	s.Text = headline
	s.UpdatedTick = nowTick
	s.UpdatedBy = "WORLD"
}

func (w *World) spawnFloodWarning(nowTick uint64, center Vec3i) {
	water, ok := w.catalogs.Blocks.Index["WATER"]
	if !ok {
		return
	}
	for _, pp := range spawnspkg.Square(spawnspkg.Pos{X: center.X, Y: 0, Z: center.Z}, 2) {
		p := Vec3i{X: pp.X, Y: pp.Y, Z: pp.Z}
		from := w.chunks.GetBlock(p)
		w.chunks.SetBlock(p, water)
		w.auditSetBlock(nowTick, "WORLD", p, from, water, "EVENT:FLOOD_WARNING")
	}
}

func (w *World) spawnBlightZone(nowTick uint64, center Vec3i) {
	gravel, ok := w.catalogs.Blocks.Index["GRAVEL"]
	if !ok {
		return
	}
	for _, pp := range spawnspkg.Diamond(spawnspkg.Pos{X: center.X, Y: 0, Z: center.Z}, 4) {
		dx := pp.X - center.X
		dz := pp.Z - center.Z
		if mathx.AbsInt(dx) > 3 || mathx.AbsInt(dz) > 3 {
			continue // keep legacy footprint
		}
		p := Vec3i{X: pp.X, Y: pp.Y, Z: pp.Z}
		from := w.chunks.GetBlock(p)
		w.chunks.SetBlock(p, gravel)
		w.auditSetBlock(nowTick, "WORLD", p, from, gravel, "EVENT:BLIGHT_ZONE")
	}
}

func (w *World) spawnBanditCamp(nowTick uint64, center Vec3i) {
	brick, okB := w.catalogs.Blocks.Index["BRICK"]
	chest, okC := w.catalogs.Blocks.Index["CHEST"]
	sign, okS := w.catalogs.Blocks.Index["SIGN"]
	if !okB || !okC || !okS {
		return
	}

	p0 := Vec3i{X: center.X, Y: 0, Z: center.Z}

	// Build a simple camp ring with a loot chest in the center.
	for _, pp := range spawnspkg.Square(spawnspkg.Pos{X: p0.X, Y: p0.Y, Z: p0.Z}, 2) {
		p := Vec3i{X: pp.X, Y: pp.Y, Z: pp.Z}
		dx := pp.X - p0.X
		dz := pp.Z - p0.Z
		from := w.chunks.GetBlock(p)
		to := w.chunks.gen.Air
		if dx == 0 && dz == 0 {
			to = chest
		} else if mathx.AbsInt(dx) == 2 || mathx.AbsInt(dz) == 2 {
			to = brick
		}
		w.chunks.SetBlock(p, to)
		w.auditSetBlock(nowTick, "WORLD", p, from, to, "EVENT:BANDIT_CAMP")
		if dx == 0 && dz == 0 {
			c := w.ensureContainer(p, "CHEST")
			c.Inventory["IRON_INGOT"] += 6
			c.Inventory["COPPER_INGOT"] += 4
			c.Inventory["CRYSTAL_SHARD"] += 1
			c.Inventory["BREAD"] += 2
		}
	}

	// Sign marker.
	sp := Vec3i{X: p0.X + 3, Y: p0.Y, Z: p0.Z}
	fromS := w.chunks.GetBlock(sp)
	w.chunks.SetBlock(sp, sign)
	w.auditSetBlock(nowTick, "WORLD", sp, fromS, sign, "EVENT:BANDIT_CAMP")
	s := w.ensureSign(sp)
	s.Text = "BANDIT CAMP"
	s.UpdatedTick = nowTick
	s.UpdatedBy = "WORLD"

	// Use chest position as the event center marker.
	w.activeEventCenter = p0
}

func (w *World) maybeSeasonRollover(nowTick uint64) {
	seasonLen := uint64(w.cfg.ResetEveryTicks)
	if seasonLen == 0 {
		seasonLen = uint64(w.cfg.SeasonLengthTicks)
	}
	if seasonLen == 0 || nowTick == 0 || nowTick%seasonLen != 0 {
		return
	}

	endedSeason := int(nowTick / seasonLen)
	newSeason := endedSeason + 1
	archiveTick := nowTick - 1

	// Force an end-of-season snapshot (best-effort, but we block to avoid silently losing archives).
	if w.snapshotSink != nil {
		snap := w.ExportSnapshot(archiveTick)
		w.snapshotSink <- snap
	}

	// Reset world state for the new season (keep cultural assets like org metadata).
	w.resetWorldForNewSeason(nowTick, newSeason, archiveTick)
}

func (w *World) seasonIndex(nowTick uint64) int {
	seasonLen := uint64(w.cfg.ResetEveryTicks)
	if seasonLen == 0 {
		seasonLen = uint64(w.cfg.SeasonLengthTicks)
	}
	if seasonLen == 0 {
		return 1
	}
	return int(nowTick/seasonLen) + 1
}

func (w *World) seasonDay(nowTick uint64) int {
	dayTicks := uint64(w.cfg.DayTicks)
	if dayTicks == 0 {
		return 1
	}
	seasonLen := uint64(w.cfg.ResetEveryTicks)
	if seasonLen == 0 {
		seasonLen = uint64(w.cfg.SeasonLengthTicks)
	}
	seasonDays := seasonLen / dayTicks
	if seasonDays == 0 {
		seasonDays = 1
	}
	return int((nowTick/dayTicks)%seasonDays) + 1
}

func (w *World) maybeWorldResetNotice(nowTick uint64) {
	cycle := uint64(w.cfg.ResetEveryTicks)
	notice := uint64(w.cfg.ResetNoticeTicks)
	if cycle == 0 || notice == 0 || notice >= cycle || nowTick == 0 {
		return
	}
	if nowTick%cycle != cycle-notice {
		return
	}
	resetTick := nowTick + notice
	for _, a := range w.agents {
		a.AddEvent(protocol.Event{
			"t":          nowTick,
			"type":       "WORLD_RESET_NOTICE",
			"world_id":   w.cfg.ID,
			"reset_tick": resetTick,
			"in_ticks":   notice,
		})
	}
}

func (w *World) resetWorldForNewSeason(nowTick uint64, newSeason int, archiveTick uint64) {
	w.resetTotal++

	// Advance world seed to reshuffle resources deterministically.
	w.cfg.Seed++

	// Reset terrain/chunks with the new seed.
	gen := w.chunks.gen
	gen.Seed = w.cfg.Seed
	w.chunks = NewChunkStore(gen)

	// Reset world-scoped mutable state.
	w.weather = "CLEAR"
	w.weatherUntilTick = 0
	w.activeEventID = ""
	w.activeEventStart = 0
	w.activeEventEnds = 0
	w.activeEventCenter = Vec3i{}
	w.activeEventRadius = 0

	w.claims = map[string]*LandClaim{}
	w.containers = map[Vec3i]*Container{}
	w.items = map[string]*ItemEntity{}
	w.itemsAt = map[Vec3i][]string{}
	w.trades = map[string]*Trade{}
	w.boards = map[string]*Board{}
	w.signs = map[Vec3i]*Sign{}
	w.conveyors = map[Vec3i]ConveyorMeta{}
	w.switches = map[Vec3i]bool{}
	w.contracts = map[string]*Contract{}
	w.laws = map[string]*Law{}
	w.structures = map[string]*Structure{}
	w.stats = NewWorldStats(300, 72000)

	// Organizations are treated as cultural assets: keep their identity and membership, but
	// reset treasuries to avoid carrying physical wealth across seasons.
	if len(w.orgs) > 0 {
		orgIDs := make([]string, 0, len(w.orgs))
		for id := range w.orgs {
			orgIDs = append(orgIDs, id)
		}
		sort.Strings(orgIDs)
		for _, id := range orgIDs {
			o := w.orgs[id]
			if o == nil {
				continue
			}
			if o.TreasuryByWorld == nil {
				o.TreasuryByWorld = map[string]map[string]int{}
			}
			o.TreasuryByWorld[w.cfg.ID] = map[string]int{}
			o.Treasury = o.TreasuryByWorld[w.cfg.ID]
		}
	}

	// Reset agent physical state; keep identity, org membership, reputation, and memory.
	agents := w.sortedAgents()
	for _, a := range agents {
		if a == nil {
			continue
		}
		w.resetAgentForNewSeason(nowTick, a)
		a.AddEvent(protocol.Event{
			"t":            nowTick,
			"type":         "SEASON_ROLLOVER",
			"season":       newSeason,
			"archive_tick": archiveTick,
			"seed":         w.cfg.Seed,
		})
		a.AddEvent(protocol.Event{
			"t":          nowTick,
			"type":       "WORLD_RESET_DONE",
			"world_id":   w.cfg.ID,
			"reset_tick": nowTick,
		})
	}
	w.auditEvent(nowTick, "SYSTEM", "WORLD_RESET", Vec3i{}, "SEASON_ROLLOVER", map[string]any{
		"world_id":     w.cfg.ID,
		"archive_tick": archiveTick,
		"season":       newSeason,
		"new_seed":     w.cfg.Seed,
	})
}

func (w *World) resetAgentForNewSeason(nowTick uint64, a *Agent) {
	// Cancel ongoing tasks.
	a.MoveTask = nil
	a.WorkTask = nil

	// Reset physical attributes.
	a.HP = 20
	a.Hunger = 20
	a.StaminaMilli = 1000
	a.Yaw = 0

	// Reset inventory to starter kit; preserve memory and reputation.
	a.Inventory = map[string]int{
		"PLANK":   20,
		"COAL":    10,
		"STONE":   20,
		"BERRIES": 10,
	}

	// Reset equipment (MVP).
	a.Equipment = Equipment{MainHand: "NONE", Armor: [4]string{"NONE", "NONE", "NONE", "NONE"}}

	// Clear ephemeral queues.
	a.Events = nil
	a.PendingMemory = nil

	// Reset anti-exploit windows so novelty/fun can be earned per season.
	a.ResetRateLimits()
	a.ResetFunTracking()
	a.Fun = FunScore{}

	// Respawn at deterministic spawn point (depends on agent number) on new terrain.
	n := respawnpkg.AgentNumber(a.ID)
	spawnXZ := n * 2
	spawnX := spawnXZ
	spawnZ := -spawnXZ
	spawnX, spawnZ = w.findSpawnAir(spawnX, spawnZ, 8)
	a.Pos = Vec3i{X: spawnX, Y: 0, Z: spawnZ}

	// Award novelty for the first biome arrival in the season.
	w.funOnBiome(a, nowTick)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func socialFunFactor(a *Agent) float64 {
	rep := a.RepTrade
	if rep >= 500 {
		return 1.0
	}
	if rep <= 0 {
		return 0.5
	}
	return 0.5 + 0.5*(float64(rep)/500.0)
}

func itoaU64(v uint64) string { return strconv.FormatUint(v, 10) }
func itoaI(v int) string      { return strconv.Itoa(v) }

func (w *World) funOnBiome(a *Agent, nowTick uint64) {
	if a == nil {
		return
	}
	b := biomeAt(w.cfg.Seed, a.Pos.X, a.Pos.Z, w.cfg.BiomeRegionSize)
	if b == "" {
		return
	}
	if !a.MarkBiomeSeen(b) {
		return
	}
	w.addFun(a, nowTick, "NOVELTY", "biome:"+b, 10)
}

func (w *World) funOnRecipe(a *Agent, recipeID string, tier int, nowTick uint64) {
	if a == nil || recipeID == "" {
		return
	}
	if !a.MarkRecipeSeen(recipeID) {
		return
	}
	pts := 3
	switch tier {
	case 0, 1:
		pts = 3
	case 2:
		pts = 5
	default:
		pts = 8
	}
	w.addFun(a, nowTick, "NOVELTY", "recipe:"+recipeID, pts)
}

func (w *World) funOnWorldEventParticipation(a *Agent, eventID string, nowTick uint64) {
	if a == nil || eventID == "" {
		return
	}
	if !a.MarkEventSeen(eventID) {
		return
	}
	w.addFun(a, nowTick, "NOVELTY", "event:"+eventID, 5)
}

func (w *World) funOnTrade(a *Agent, nowTick uint64) {
	if a == nil {
		return
	}
	base := 2
	// Low reputation makes social fun less valuable (anti-scam signal).
	factor := socialFunFactor(a)
	base = int(math.Round(float64(base) * factor))
	if base <= 0 {
		return
	}
	w.addFun(a, nowTick, "SOCIAL", "trade", a.FunDecayDelta("social:trade", base, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
}

func (w *World) funOnContractComplete(a *Agent, nowTick uint64, kind string) {
	if a == nil {
		return
	}
	base := 5
	if kind == "BUILD" {
		base = 7
	}
	factor := socialFunFactor(a)
	base = int(math.Round(float64(base) * factor))
	if base <= 0 {
		return
	}
	w.addFun(a, nowTick, "SOCIAL", "contract", a.FunDecayDelta("social:contract", base, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	if w.activeEventID != "" && nowTick < w.activeEventEnds {
		w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
		w.addFun(a, nowTick, "NARRATIVE", "event_success", a.FunDecayDelta("narrative:event_success", 5, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	}
	if w.weather == "STORM" || w.weather == "COLD" {
		w.addFun(a, nowTick, "RISK_RESCUE", "hazard_success", a.FunDecayDelta("risk:hazard_success", 8, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	}
}

func (w *World) funOnBlueprintComplete(a *Agent, nowTick uint64) {
	if a == nil {
		return
	}
	if w.activeEventID != "" && nowTick < w.activeEventEnds {
		w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
		w.addFun(a, nowTick, "NARRATIVE", "event_build", a.FunDecayDelta("narrative:event_build", 5, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	}
	if w.weather == "STORM" || w.weather == "COLD" {
		w.addFun(a, nowTick, "RISK_RESCUE", "hazard_build", a.FunDecayDelta("risk:hazard_build", 8, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	}
}

func (w *World) funOnLawActive(proposer *Agent, nowTick uint64) {
	if proposer == nil {
		return
	}
	w.addFun(proposer, nowTick, "INFLUENCE", "law_adopted", proposer.FunDecayDelta("influence:law_adopted", 4, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	w.addFun(proposer, nowTick, "NARRATIVE", "law_adopted", proposer.FunDecayDelta("narrative:law_adopted", 5, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	if w.activeEventID == "CIVIC_VOTE" && nowTick < w.activeEventEnds {
		w.funOnWorldEventParticipation(proposer, w.activeEventID, nowTick)
		w.addFun(proposer, nowTick, "NARRATIVE", "civic_vote_law", proposer.FunDecayDelta("narrative:civic_vote_law", 6, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
	}
}

func (w *World) funOnVote(a *Agent, nowTick uint64) {
	if a == nil {
		return
	}
	base := 2
	key := "narrative:vote"
	reason := "vote"
	if w.activeEventID == "CIVIC_VOTE" && nowTick < w.activeEventEnds {
		w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
		base = 4
		key = "narrative:civic_vote_vote"
		reason = "civic_vote_vote"
	}
	w.addFun(a, nowTick, "NARRATIVE", reason, a.FunDecayDelta(key, base, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
}

func (w *World) addFun(a *Agent, nowTick uint64, dim string, reason string, delta int) {
	if a == nil || delta == 0 {
		return
	}
	total := 0
	switch dim {
	case "NOVELTY":
		a.Fun.Novelty += delta
		total = a.Fun.Novelty
	case "CREATION":
		a.Fun.Creation += delta
		total = a.Fun.Creation
	case "SOCIAL":
		a.Fun.Social += delta
		total = a.Fun.Social
	case "INFLUENCE":
		a.Fun.Influence += delta
		total = a.Fun.Influence
	case "NARRATIVE":
		a.Fun.Narrative += delta
		total = a.Fun.Narrative
	case "RISK_RESCUE":
		a.Fun.RiskRescue += delta
		total = a.Fun.RiskRescue
	default:
		return
	}

	a.AddEvent(protocol.Event{
		"t":      nowTick,
		"type":   "FUN",
		"dim":    dim,
		"delta":  delta,
		"total":  total,
		"reason": reason,
	})
}

func (w *World) funInit() {
	if w.structures == nil {
		w.structures = map[string]*Structure{}
	}
}

func (w *World) systemFun(nowTick uint64) {
	w.funInit()

	// Award delayed creation scores.
	if len(w.structures) > 0 {
		ids := make([]string, 0, len(w.structures))
		for id := range w.structures {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			s := w.structures[id]
			if s == nil || s.Awarded || nowTick < s.AwardDueTick {
				continue
			}

			// Validate that the structure still exists and matches the blueprint.
			if !w.checkBlueprintPlaced(s.BlueprintID, s.Anchor, s.Rotation) {
				delete(w.structures, id)
				continue
			}

			builder := w.agents[s.BuilderID]
			if builder == nil {
				delete(w.structures, id)
				continue
			}

			bp, ok := w.catalogs.Blueprints.ByID[s.BlueprintID]
			if !ok {
				delete(w.structures, id)
				continue
			}

			creationPts := w.structureCreationScore(&bp, s, nowTick)
			if creationPts > 0 {
				w.addFun(builder, nowTick, "CREATION", "structure", builder.FunDecayDelta("creation:structure", creationPts, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
			}

			s.Awarded = true
		}
	}

	// Influence: award per in-game day boundary.
	if w.cfg.DayTicks > 0 && nowTick != 0 && nowTick%uint64(w.cfg.DayTicks) == 0 {
		day := int(nowTick / uint64(w.cfg.DayTicks))
		ids := make([]string, 0, len(w.structures))
		for id := range w.structures {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			s := w.structures[id]
			if s == nil {
				continue
			}
			if !w.checkBlueprintPlaced(s.BlueprintID, s.Anchor, s.Rotation) {
				delete(w.structures, id)
				continue
			}
			if s.LastInfluenceDay == day {
				continue
			}
			s.LastInfluenceDay = day
			builder := w.agents[s.BuilderID]
			if builder == nil {
				continue
			}
			users := w.structureUniqueUsers(s, nowTick, uint64(w.cfg.DayTicks))
			if users <= 0 {
				continue
			}
			pts := int(math.Round(minFloat(15, 3*math.Sqrt(float64(users)))))
			if pts > 0 {
				w.addFun(builder, nowTick, "INFLUENCE", "infra_usage_day", builder.FunDecayDelta("influence:infra_usage_day", pts, nowTick, uint64(w.cfg.FunDecayWindowTicks), w.cfg.FunDecayBase))
			}
		}
	}
}

func (w *World) registerStructure(nowTick uint64, builderID string, blueprintID string, anchor Vec3i, rotation int) {
	w.funInit()
	bp, ok := w.catalogs.Blueprints.ByID[blueprintID]
	if !ok {
		return
	}
	rot := blueprint.NormalizeRotation(rotation)

	id := fmtStructureID(builderID, nowTick, blueprintID, anchor)

	// Compute actual bounds from rotated block positions (rotation affects Min/Max).
	min := Vec3i{X: anchor.X, Y: anchor.Y, Z: anchor.Z}
	max := Vec3i{X: anchor.X, Y: anchor.Y, Z: anchor.Z}
	if len(bp.Blocks) > 0 {
		first := true
		for _, b := range bp.Blocks {
			off := blueprint.RotateOffset(b.Pos, rot)
			p := Vec3i{X: anchor.X + off[0], Y: anchor.Y + off[1], Z: anchor.Z + off[2]}
			if first {
				min, max = p, p
				first = false
				continue
			}
			if p.X < min.X {
				min.X = p.X
			}
			if p.Y < min.Y {
				min.Y = p.Y
			}
			if p.Z < min.Z {
				min.Z = p.Z
			}
			if p.X > max.X {
				max.X = p.X
			}
			if p.Y > max.Y {
				max.Y = p.Y
			}
			if p.Z > max.Z {
				max.Z = p.Z
			}
		}
	}
	w.structures[id] = &Structure{
		StructureID:   id,
		BlueprintID:   blueprintID,
		BuilderID:     builderID,
		Anchor:        anchor,
		Rotation:      rot,
		Min:           min,
		Max:           max,
		CompletedTick: nowTick,
		AwardDueTick:  nowTick + uint64(w.cfg.StructureSurvivalTicks),
		UsedBy:        map[string]uint64{},
	}
}

func fmtStructureID(builderID string, nowTick uint64, blueprintID string, anchor Vec3i) string {
	// Deterministic, stable id (no counters) so snapshots and replays match.
	return "STRUCT_" + builderID + "_" + itoaU64(nowTick) + "_" + blueprintID + "_" + itoaI(anchor.X) + "_" + itoaI(anchor.Y) + "_" + itoaI(anchor.Z)
}

func (w *World) recordStructureUsage(agentID string, pos Vec3i, nowTick uint64) {
	if len(w.structures) == 0 || agentID == "" {
		return
	}
	for _, s := range w.structures {
		if s == nil {
			continue
		}
		if pos.X < s.Min.X || pos.X > s.Max.X || pos.Y < s.Min.Y || pos.Y > s.Max.Y || pos.Z < s.Min.Z || pos.Z > s.Max.Z {
			continue
		}
		if s.UsedBy == nil {
			s.UsedBy = map[string]uint64{}
		}
		s.UsedBy[agentID] = nowTick
	}
}

func (w *World) structureUniqueUsers(s *Structure, nowTick uint64, window uint64) int {
	if s == nil || len(s.UsedBy) == 0 {
		return 0
	}
	cutoff := uint64(0)
	if nowTick > window {
		cutoff = nowTick - window
	}
	n := 0
	for aid, last := range s.UsedBy {
		if aid == "" || aid == s.BuilderID {
			continue
		}
		if last >= cutoff {
			n++
		}
	}
	return n
}

func (w *World) structureCreationScore(bp *catalogs.BlueprintDef, s *Structure, nowTick uint64) int {
	if bp == nil || s == nil {
		return 0
	}
	unique := map[string]bool{}
	hasStorage := false
	hasLight := false
	hasWorkshop := false
	hasGov := false

	for _, b := range bp.Blocks {
		unique[b.Block] = true
		switch b.Block {
		case "CHEST":
			hasStorage = true
		case "TORCH":
			hasLight = true
		case "CRAFTING_BENCH", "FURNACE":
			hasWorkshop = true
		case "BULLETIN_BOARD", "CONTRACT_TERMINAL", "CLAIM_TOTEM", "SIGN":
			hasGov = true
		}
	}

	base := 5
	complexity := int(math.Round(math.Log(1+float64(len(unique))) * 2))
	modules := 0
	if hasStorage {
		modules += 2
	}
	if hasLight {
		modules += 2
	}
	if hasWorkshop {
		modules += 2
	}
	if hasGov {
		modules += 2
	}

	stable := w.structureStable(bp, s.Anchor, s.Rotation)
	stability := 0
	if stable {
		stability = 3
	}

	users := w.structureUniqueUsers(s, nowTick, uint64(w.cfg.DayTicks))
	usageBonus := minInt(10, 2*users)

	return base + complexity + modules + stability + usageBonus
}

func (w *World) structureStable(bp *catalogs.BlueprintDef, anchor Vec3i, rotation int) bool {
	if bp == nil || len(bp.Blocks) == 0 {
		return true
	}
	rot := blueprint.NormalizeRotation(rotation)
	positions := make([]Vec3i, 0, len(bp.Blocks))
	index := map[Vec3i]int{}
	for i, b := range bp.Blocks {
		off := blueprint.RotateOffset(b.Pos, rot)
		p := Vec3i{X: anchor.X + off[0], Y: anchor.Y + off[1], Z: anchor.Z + off[2]}
		positions = append(positions, p)
		index[p] = i
	}
	visited := make([]bool, len(positions))
	queue := make([]int, 0, len(positions))

	// Seed BFS with blocks that have ground support (non-air below).
	for i, p := range positions {
		if p.Y <= 1 {
			visited[i] = true
			queue = append(queue, i)
			continue
		}
		below := Vec3i{X: p.X, Y: p.Y - 1, Z: p.Z}
		if _, ok := index[below]; ok {
			// Supported by structure; not ground.
			continue
		}
		if w.chunks.GetBlock(below) != w.chunks.gen.Air {
			visited[i] = true
			queue = append(queue, i)
		}
	}

	dirs := []Vec3i{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		p := positions[i]
		for _, d := range dirs {
			np := Vec3i{X: p.X + d.X, Y: p.Y + d.Y, Z: p.Z + d.Z}
			ni, ok := index[np]
			if !ok || visited[ni] {
				continue
			}
			visited[ni] = true
			queue = append(queue, ni)
		}
	}

	count := 0
	for _, v := range visited {
		if v {
			count++
		}
	}
	if count == 0 {
		return false
	}
	return float64(count)/float64(len(visited)) >= 0.95
}
