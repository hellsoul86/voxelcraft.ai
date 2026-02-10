package world

import (
	"math"
	"sort"

	"voxelcraft.ai/internal/protocol"
)

type directorMetrics struct {
	Trade       float64 // 0..1
	Conflict    float64 // 0..1
	Exploration float64 // 0..1
	Inequality  float64 // 0..1 (Gini)
	PublicInfra float64 // 0..1
}

func (w *World) systemDirector(nowTick uint64) {
	// Expire active event.
	if w.activeEventEnds != 0 && nowTick >= w.activeEventEnds {
		w.activeEventID = ""
		w.activeEventStart = 0
		w.activeEventEnds = 0
		w.activeEventCenter = Vec3i{}
		w.activeEventRadius = 0
	}
	// Expire weather override.
	if w.weatherUntilTick != 0 && nowTick >= w.weatherUntilTick {
		w.weather = "CLEAR"
		w.weatherUntilTick = 0
	}

	// If an event is still active, don't schedule a new one.
	if w.activeEventID != "" {
		return
	}

	// First-week scripted cadence at the start of each in-game day.
	if w.cfg.DayTicks > 0 && nowTick%uint64(w.cfg.DayTicks) == 0 {
		schedule := []string{
			"MARKET_WEEK",
			"CRYSTAL_RIFT",
			"BUILDER_EXPO",
			"FLOOD_WARNING",
			"RUINS_GATE",
			"BANDIT_CAMP",
			"CIVIC_VOTE",
		}
		dayInSeason := w.seasonDay(nowTick)
		if dayInSeason >= 1 && dayInSeason <= len(schedule) {
			w.startEvent(nowTick, schedule[dayInSeason-1])
			return
		}
	}

	// After week 1, evaluate every N ticks (default 3000 ~= 10 minutes at 5Hz).
	every := uint64(w.cfg.DirectorEveryTicks)
	if every == 0 {
		every = 3000
	}
	if nowTick == 0 || nowTick%every != 0 {
		return
	}

	m := w.computeDirectorMetrics(nowTick)
	weights := w.baseEventWeights()

	// Feedback rules (match the spec's intent; numbers are tunable).
	if m.Trade < 0.4 {
		weights["MARKET_WEEK"] += 0.25
		weights["BLUEPRINT_FAIR"] += 0.15
	}
	if m.Exploration < 0.3 {
		weights["CRYSTAL_RIFT"] += 0.20
		weights["RUINS_GATE"] += 0.20
	}
	if m.Conflict < 0.10 {
		weights["DEEP_VEIN"] += 0.15
		weights["BANDIT_CAMP"] += 0.10
	} else if m.Conflict > 0.25 {
		weights["CIVIC_VOTE"] += 0.25
		weights["MARKET_WEEK"] += 0.10
		weights["BUILDER_EXPO"] += 0.10
	}
	if m.Inequality > 0.50 {
		weights["CIVIC_VOTE"] += 0.20
		weights["FLOOD_WARNING"] += 0.10
	}

	// Sample deterministically using world seed + tick.
	ev := sampleWeighted(weights, hash2(w.cfg.Seed, int(nowTick), 1337))
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

func sampleWeighted(weights map[string]float64, roll uint64) string {
	if len(weights) == 0 {
		return ""
	}
	ids := make([]string, 0, len(weights))
	var total float64
	for id, w := range weights {
		if w > 0 {
			ids = append(ids, id)
			total += w
		}
	}
	if total <= 0 || len(ids) == 0 {
		return ""
	}
	sort.Strings(ids)

	// Deterministic pick in [0,total).
	r := float64(roll%1_000_000_000) / 1_000_000_000.0
	target := r * total

	var acc float64
	for _, id := range ids {
		acc += weights[id]
		if target <= acc {
			return id
		}
	}
	return ids[len(ids)-1]
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
	boundary := w.cfg.BoundaryR
	if boundary <= 0 {
		boundary = 4000
	}
	margin := 64
	span := boundary*2 - margin*2
	if span <= 0 {
		margin = 0
		span = boundary * 2
		if span <= 0 {
			span = 1
		}
	}

	eh := hashEventID(eventID)
	for attempt := 0; attempt < 32; attempt++ {
		hx := hash3(w.cfg.Seed, eh, int(nowTick), attempt*2)
		hz := hash3(w.cfg.Seed, eh, int(nowTick), attempt*2+1)
		x := -boundary + margin + int(hx%uint64(span))
		z := -boundary + margin + int(hz%uint64(span))

		y := w.surfaceY(x, z)
		p := Vec3i{X: x, Y: y, Z: z}
		// Avoid placing event centers inside claimed land.
		if w.landAt(p) != nil {
			continue
		}
		return p
	}
	// Fallback (deterministic).
	y := w.surfaceY(0, 0)
	return Vec3i{X: 0, Y: y, Z: 0}
}

func hashEventID(id string) int {
	// FNV-1a 64-bit, folded to int.
	var h uint64 = 1469598103934665603
	for i := 0; i < len(id); i++ {
		h ^= uint64(id[i])
		h *= 1099511628211
	}
	return int(uint32(h))
}

func distXZ(a, b Vec3i) int {
	return abs(a.X-b.X) + abs(a.Z-b.Z)
}

func (w *World) spawnCrystalRift(nowTick uint64, center Vec3i) {
	ore, ok := w.catalogs.Blocks.Index["CRYSTAL_ORE"]
	if !ok {
		return
	}
	// Spawn a compact underground cluster below the surface marker.
	yc := 8
	if yc < 2 {
		yc = 2
	}
	if yc >= w.cfg.Height-1 {
		yc = w.cfg.Height - 2
	}
	c := Vec3i{X: center.X, Y: yc, Z: center.Z}

	for dy := -1; dy <= 1; dy++ {
		for dz := -2; dz <= 2; dz++ {
			for dx := -2; dx <= 2; dx++ {
				p := Vec3i{X: c.X + dx, Y: c.Y + dy, Z: c.Z + dz}
				from := w.chunks.GetBlock(p)
				w.chunks.SetBlock(p, ore)
				w.auditSetBlock(nowTick, "WORLD", p, from, ore, "EVENT:CRYSTAL_RIFT")
			}
		}
	}
}

func (w *World) spawnDeepVein(nowTick uint64, center Vec3i) {
	iron, ok1 := w.catalogs.Blocks.Index["IRON_ORE"]
	copper, ok2 := w.catalogs.Blocks.Index["COPPER_ORE"]
	if !ok1 || !ok2 {
		return
	}
	yc := 6
	if yc < 2 {
		yc = 2
	}
	if yc >= w.cfg.Height-1 {
		yc = w.cfg.Height - 2
	}
	c := Vec3i{X: center.X, Y: yc, Z: center.Z}

	for dy := -2; dy <= 2; dy++ {
		for dz := -3; dz <= 3; dz++ {
			for dx := -3; dx <= 3; dx++ {
				p := Vec3i{X: c.X + dx, Y: c.Y + dy, Z: c.Z + dz}
				to := iron
				if (dx+dz+dy)&1 == 0 {
					to = copper
				}
				from := w.chunks.GetBlock(p)
				w.chunks.SetBlock(p, to)
				w.auditSetBlock(nowTick, "WORLD", p, from, to, "EVENT:DEEP_VEIN")
			}
		}
	}
}

func (w *World) spawnRuinsGate(nowTick uint64, center Vec3i) {
	brick, okB := w.catalogs.Blocks.Index["BRICK"]
	chest, okC := w.catalogs.Blocks.Index["CHEST"]
	if !okB || !okC {
		return
	}

	// Build a small ring at the surface with a loot chest in the center.
	y := w.surfaceY(center.X, center.Z)
	p0 := Vec3i{X: center.X, Y: y, Z: center.Z}

	for dz := -1; dz <= 1; dz++ {
		for dx := -1; dx <= 1; dx++ {
			p := Vec3i{X: p0.X + dx, Y: p0.Y, Z: p0.Z + dz}
			from := w.chunks.GetBlock(p)
			to := brick
			if dx == 0 && dz == 0 {
				to = chest
			}
			w.chunks.SetBlock(p, to)
			w.auditSetBlock(nowTick, "WORLD", p, from, to, "EVENT:RUINS_GATE")
			if dx == 0 && dz == 0 {
				c := w.ensureContainer(p, "CHEST")
				c.Inventory["CRYSTAL_SHARD"] += 2
				c.Inventory["IRON_INGOT"] += 4
				c.Inventory["COPPER_INGOT"] += 4
			}
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

	y := w.surfaceY(center.X, center.Z)
	boardPos := Vec3i{X: center.X, Y: y, Z: center.Z}
	signPos := Vec3i{X: center.X + 1, Y: y, Z: center.Z}

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
	y := w.surfaceY(center.X, center.Z)
	for dz := -2; dz <= 2; dz++ {
		for dx := -2; dx <= 2; dx++ {
			p := Vec3i{X: center.X + dx, Y: y, Z: center.Z + dz}
			from := w.chunks.GetBlock(p)
			w.chunks.SetBlock(p, water)
			w.auditSetBlock(nowTick, "WORLD", p, from, water, "EVENT:FLOOD_WARNING")
		}
	}
}

func (w *World) spawnBlightZone(nowTick uint64, center Vec3i) {
	gravel, ok := w.catalogs.Blocks.Index["GRAVEL"]
	if !ok {
		return
	}
	groundY := w.surfaceY(center.X, center.Z) - 1
	if groundY < 1 {
		groundY = 1
	}
	for dz := -3; dz <= 3; dz++ {
		for dx := -3; dx <= 3; dx++ {
			if abs(dx)+abs(dz) > 4 {
				continue
			}
			p := Vec3i{X: center.X + dx, Y: groundY, Z: center.Z + dz}
			from := w.chunks.GetBlock(p)
			w.chunks.SetBlock(p, gravel)
			w.auditSetBlock(nowTick, "WORLD", p, from, gravel, "EVENT:BLIGHT_ZONE")
		}
	}
}

func (w *World) spawnBanditCamp(nowTick uint64, center Vec3i) {
	brick, okB := w.catalogs.Blocks.Index["BRICK"]
	chest, okC := w.catalogs.Blocks.Index["CHEST"]
	sign, okS := w.catalogs.Blocks.Index["SIGN"]
	if !okB || !okC || !okS {
		return
	}

	y := w.surfaceY(center.X, center.Z)
	p0 := Vec3i{X: center.X, Y: y, Z: center.Z}

	// Build a simple camp ring with a loot chest in the center.
	for dz := -2; dz <= 2; dz++ {
		for dx := -2; dx <= 2; dx++ {
			p := Vec3i{X: p0.X + dx, Y: p0.Y, Z: p0.Z + dz}
			from := w.chunks.GetBlock(p)
			to := w.chunks.gen.Air
			if dx == 0 && dz == 0 {
				to = chest
			} else if abs(dx) == 2 || abs(dz) == 2 {
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

func (w *World) onMinedBlockDuringEvent(a *Agent, pos Vec3i, blockName string, nowTick uint64) {
	if a == nil || blockName == "" || w.activeEventID == "" || w.activeEventRadius <= 0 {
		return
	}
	if distXZ(pos, w.activeEventCenter) > w.activeEventRadius {
		return
	}
	switch w.activeEventID {
	case "CRYSTAL_RIFT":
		if blockName != "CRYSTAL_ORE" {
			return
		}
		// Bonus shard to make the expedition feel rewarding.
		a.Inventory["CRYSTAL_SHARD"]++
		w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
		w.addFun(a, nowTick, "NARRATIVE", "event_mine", w.funDecay(a, "narrative:event_mine:"+w.activeEventID, 5, nowTick))
		a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "MINE_CRYSTAL"})

	case "DEEP_VEIN":
		// Bonus ore in the event zone.
		switch blockName {
		case "IRON_ORE":
			a.Inventory["IRON_ORE"]++
		case "COPPER_ORE":
			a.Inventory["COPPER_ORE"]++
		default:
			return
		}
		w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
		w.addFun(a, nowTick, "NARRATIVE", "event_mine", w.funDecay(a, "narrative:event_mine:"+w.activeEventID, 5, nowTick))
		a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "MINE_VEIN"})
	}
}

func (w *World) onContainerOpenedDuringEvent(a *Agent, c *Container, nowTick uint64) {
	if a == nil || c == nil || w.activeEventID == "" || w.activeEventRadius <= 0 {
		return
	}
	if distXZ(c.Pos, w.activeEventCenter) > w.activeEventRadius {
		return
	}
	switch w.activeEventID {
	case "RUINS_GATE":
		if c.Type != "CHEST" {
			return
		}
		w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
		w.addFun(a, nowTick, "NARRATIVE", "ruins_open", w.funDecay(a, "narrative:ruins_open", 12, nowTick))
		a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "OPEN_RUINS"})

	case "BANDIT_CAMP":
		if c.Type != "CHEST" {
			return
		}
		w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
		w.addFun(a, nowTick, "RISK_RESCUE", "bandit_loot", w.funDecay(a, "risk:bandit_loot", 10, nowTick))
		w.addFun(a, nowTick, "NARRATIVE", "bandit_loot", w.funDecay(a, "narrative:bandit_loot", 8, nowTick))
		a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "LOOT_BANDITS"})
	}
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

	tradePerAgent := float64(sum.Trades) / float64(agents)
	trade := min1(tradePerAgent / 5.0)

	deniedPerTickPerAgent := float64(sum.Denied) / float64(uint64(agents)*windowTicks)
	conflict := min1(deniedPerTickPerAgent * 100.0) // ~0.1 when ~1 denied / 1000 ticks / agent

	chunksPerAgent := float64(sum.ChunksDiscovered) / float64(agents)
	exploration := min1(chunksPerAgent / 20.0)

	infraPerAgent := float64(sum.BlueprintsComplete) / float64(agents)
	publicInfra := min1(infraPerAgent / 5.0)

	inequality := giniWealth(w.sortedAgents())

	return directorMetrics{
		Trade:       trade,
		Conflict:    conflict,
		Exploration: exploration,
		Inequality:  inequality,
		PublicInfra: publicInfra,
	}
}

func giniWealth(agents []*Agent) float64 {
	if len(agents) <= 1 {
		return 0
	}
	values := make([]float64, 0, len(agents))
	var sum float64
	for _, a := range agents {
		if a == nil {
			continue
		}
		v := wealthValue(a.Inventory)
		values = append(values, v)
		sum += v
	}
	if len(values) <= 1 || sum <= 0 {
		return 0
	}
	sort.Float64s(values)

	// Gini coefficient: (2*sum_i i*x_i)/(n*sum x) - (n+1)/n, with i=1..n.
	n := float64(len(values))
	var weighted float64
	for i, x := range values {
		weighted += float64(i+1) * x
	}
	g := (2.0*weighted)/(n*sum) - (n+1.0)/n
	if g < 0 {
		return 0
	}
	if g > 1 {
		return 1
	}
	return g
}

func wealthValue(inv map[string]int) float64 {
	if len(inv) == 0 {
		return 0
	}
	var v float64
	for item, n := range inv {
		if n <= 0 {
			continue
		}
		v += float64(n) * itemUnitValue(item)
	}
	return v
}

func itemUnitValue(item string) float64 {
	switch item {
	case "CRYSTAL_SHARD":
		return 50
	case "IRON_INGOT":
		return 10
	case "COPPER_INGOT":
		return 6
	case "COAL":
		return 1
	case "PLANK":
		return 1
	default:
		// Default weight for unknown items to keep inequality defined.
		return 0.5
	}
}

func min1(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 || math.IsNaN(x) {
		return 1
	}
	return x
}
