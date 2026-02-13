package world

import (
	"sort"
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	eventspkg "voxelcraft.ai/internal/sim/world/feature/director/events"
	feedbackpkg "voxelcraft.ai/internal/sim/world/feature/director/feedback"
	metricspkg "voxelcraft.ai/internal/sim/world/feature/director/metrics"
	runtimepkg "voxelcraft.ai/internal/sim/world/feature/director/runtime"
	spawnspkg "voxelcraft.ai/internal/sim/world/feature/director/spawns"
	statspkg "voxelcraft.ai/internal/sim/world/feature/director/stats"
	respawnpkg "voxelcraft.ai/internal/sim/world/feature/survival/respawn"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
	"voxelcraft.ai/internal/sim/world/logic/directorcenter"
	"voxelcraft.ai/internal/sim/world/logic/mathx"
)

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

func (w *World) computeDirectorMetrics(nowTick uint64) metricspkg.EvalMetrics {
	agents := len(w.agents)
	if agents <= 0 {
		return metricspkg.EvalMetrics{}
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

	return m
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

	params := map[string]any{}
	if tpl, ok := w.catalogs.Events.ByID[eventID]; ok && tpl.Params != nil {
		params = tpl.Params
	}
	plan := eventspkg.BuildInstantiatePlan(eventID, params)
	if !plan.NeedsCenter {
		return
	}
	center := w.pickEventCenter(nowTick, eventID)
	w.activeEventCenter = center
	w.activeEventRadius = plan.Radius

	didNotice := false
	switch plan.Spawn {
	case eventspkg.SpawnCrystalRift:
		w.spawnCrystalRift(nowTick, center)
	case eventspkg.SpawnDeepVein:
		w.spawnDeepVein(nowTick, center)
	case eventspkg.SpawnRuinsGate:
		w.spawnRuinsGate(nowTick, center)
	case eventspkg.SpawnFloodWarning:
		w.spawnFloodWarning(nowTick, center)
	case eventspkg.SpawnBanditCamp:
		w.spawnBanditCamp(nowTick, center)
	case eventspkg.SpawnBlightZone:
		w.spawnBlightZone(nowTick, center)
	case eventspkg.SpawnNoticeBoard:
		didNotice = true
		w.spawnEventNoticeBoard(nowTick, center, eventID, plan.Headline, plan.Body)
	}
	if !didNotice && plan.Headline != "" {
		w.spawnEventNoticeBoard(nowTick, center, eventID, plan.Headline, plan.Body)
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

func (w *World) applySpawnPlan(nowTick uint64, reason string, plan spawnspkg.Plan) {
	if plan.Center != nil {
		w.activeEventCenter = Vec3i{X: plan.Center.X, Y: plan.Center.Y, Z: plan.Center.Z}
	}
	for _, p := range plan.Placements {
		to, ok := w.catalogs.Blocks.Index[p.Block]
		if !ok {
			continue
		}
		pos := Vec3i{X: p.Pos.X, Y: p.Pos.Y, Z: p.Pos.Z}
		from := w.chunks.GetBlock(pos)
		w.chunks.SetBlock(pos, to)
		w.auditSetBlock(nowTick, "WORLD", pos, from, to, reason)
	}
	for _, c := range plan.Containers {
		pos := Vec3i{X: c.Pos.X, Y: c.Pos.Y, Z: c.Pos.Z}
		container := w.ensureContainer(pos, c.Type)
		if container == nil {
			continue
		}
		for item, count := range c.Items {
			if count <= 0 {
				continue
			}
			container.Inventory[item] += count
		}
	}
	for _, s := range plan.Signs {
		pos := Vec3i{X: s.Pos.X, Y: s.Pos.Y, Z: s.Pos.Z}
		sign := w.ensureSign(pos)
		sign.Text = s.Text
		sign.UpdatedTick = nowTick
		sign.UpdatedBy = "WORLD"
	}
	for _, post := range plan.BoardPosts {
		pos := Vec3i{X: post.Pos.X, Y: post.Pos.Y, Z: post.Pos.Z}
		w.ensureBoard(pos)
		if b := w.boards[boardIDAt(pos)]; b != nil {
			b.Posts = append(b.Posts, BoardPost{
				PostID: w.newPostID(),
				Author: post.Author,
				Title:  post.Title,
				Body:   post.Body,
				Tick:   nowTick,
			})
		}
	}
}

func (w *World) spawnCrystalRift(nowTick uint64, center Vec3i) {
	w.applySpawnPlan(nowTick, "EVENT:CRYSTAL_RIFT", spawnspkg.CrystalRiftPlan(spawnspkg.Pos{X: center.X, Y: 0, Z: center.Z}))
}

func (w *World) spawnDeepVein(nowTick uint64, center Vec3i) {
	w.applySpawnPlan(nowTick, "EVENT:DEEP_VEIN", spawnspkg.DeepVeinPlan(spawnspkg.Pos{X: center.X, Y: 0, Z: center.Z}))
}

func (w *World) spawnRuinsGate(nowTick uint64, center Vec3i) {
	w.applySpawnPlan(nowTick, "EVENT:RUINS_GATE", spawnspkg.RuinsGatePlan(spawnspkg.Pos{X: center.X, Y: 0, Z: center.Z}))
}

func (w *World) spawnEventNoticeBoard(nowTick uint64, center Vec3i, eventID string, headline string, body string) {
	w.applySpawnPlan(nowTick, "EVENT:"+eventID, spawnspkg.EventNoticeBoardPlan(spawnspkg.Pos{X: center.X, Y: 0, Z: center.Z}, headline, body))
}

func (w *World) spawnFloodWarning(nowTick uint64, center Vec3i) {
	w.applySpawnPlan(nowTick, "EVENT:FLOOD_WARNING", spawnspkg.FloodWarningPlan(spawnspkg.Pos{X: center.X, Y: 0, Z: center.Z}))
}

func (w *World) spawnBlightZone(nowTick uint64, center Vec3i) {
	w.applySpawnPlan(nowTick, "EVENT:BLIGHT_ZONE", spawnspkg.BlightZonePlan(spawnspkg.Pos{X: center.X, Y: 0, Z: center.Z}))
}

func (w *World) spawnBanditCamp(nowTick uint64, center Vec3i) {
	w.applySpawnPlan(nowTick, "EVENT:BANDIT_CAMP", spawnspkg.BanditCampPlan(spawnspkg.Pos{X: center.X, Y: 0, Z: center.Z}))
}

func (w *World) maybeSeasonRollover(nowTick uint64) {
	seasonLen := runtimepkg.SeasonLengthTicks(w.cfg.ResetEveryTicks, w.cfg.SeasonLengthTicks)
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
	return runtimepkg.SeasonIndex(nowTick, w.cfg.ResetEveryTicks, w.cfg.SeasonLengthTicks)
}

func (w *World) seasonDay(nowTick uint64) int {
	return runtimepkg.SeasonDay(nowTick, w.cfg.DayTicks, w.cfg.ResetEveryTicks, w.cfg.SeasonLengthTicks)
}

func (w *World) maybeWorldResetNotice(nowTick uint64) {
	ok, resetTick := runtimepkg.ShouldWorldResetNotice(nowTick, w.cfg.ResetEveryTicks, w.cfg.ResetNoticeTicks)
	if !ok {
		return
	}
	notice := uint64(w.cfg.ResetNoticeTicks)
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
	respawnpkg.ResetForSeason(a, w.findSpawnAir)
	// Award novelty for the first biome arrival in the season.
	w.funOnBiome(a, nowTick)
}

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
	base = statspkg.ScaleByFactor(base, statspkg.SocialFunFactor(a.RepTrade))
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
	base = statspkg.ScaleByFactor(base, statspkg.SocialFunFactor(a.RepTrade))
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
			pts := statspkg.InfluenceUsagePoints(users)
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

	id := statspkg.StructureID(builderID, nowTick, blueprintID, anchor.X, anchor.Y, anchor.Z)

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
	if s == nil {
		return 0
	}
	return statspkg.StructureUniqueUsers(s.UsedBy, s.BuilderID, nowTick, window)
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

	stable := w.structureStable(bp, s.Anchor, s.Rotation)
	users := w.structureUniqueUsers(s, nowTick, uint64(w.cfg.DayTicks))
	return statspkg.CreationScore(statspkg.CreationScoreInput{
		UniqueBlockTypes: len(unique),
		HasStorage:       hasStorage,
		HasLight:         hasLight,
		HasWorkshop:      hasWorkshop,
		HasGovernance:    hasGov,
		Stable:           stable,
		Users:            users,
	})
}

func (w *World) structureStable(bp *catalogs.BlueprintDef, anchor Vec3i, rotation int) bool {
	if bp == nil || len(bp.Blocks) == 0 {
		return true
	}
	rot := blueprint.NormalizeRotation(rotation)
	positions := make([]statspkg.Vec3, 0, len(bp.Blocks))
	for _, b := range bp.Blocks {
		off := blueprint.RotateOffset(b.Pos, rot)
		p := statspkg.Vec3{X: anchor.X + off[0], Y: anchor.Y + off[1], Z: anchor.Z + off[2]}
		positions = append(positions, p)
	}
	return statspkg.IsStructureStable(positions, func(x, y, z int) bool {
		return w.chunks.GetBlock(Vec3i{X: x, Y: y, Z: z}) != w.chunks.gen.Air
	})
}
