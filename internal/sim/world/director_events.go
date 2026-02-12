package world

import (
	"voxelcraft.ai/internal/protocol"
	eventspkg "voxelcraft.ai/internal/sim/world/feature/director/events"
	"voxelcraft.ai/internal/sim/world/logic/directorcenter"
	"voxelcraft.ai/internal/sim/world/logic/mathx"
)

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
		w.addFun(a, nowTick, "NARRATIVE", "event_mine", w.funDecay(a, "narrative:event_mine:"+w.activeEventID, out.Narrative, nowTick))
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
		w.addFun(a, nowTick, "RISK_RESCUE", "bandit_loot", w.funDecay(a, "risk:bandit_loot", out.Risk, nowTick))
	}
	if out.Narrative > 0 {
		w.addFun(a, nowTick, "NARRATIVE", "bandit_loot", w.funDecay(a, "narrative:bandit_loot", out.Narrative, nowTick))
	}
	if out.GoalKind != "" {
		a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": out.GoalKind})
	}
}
