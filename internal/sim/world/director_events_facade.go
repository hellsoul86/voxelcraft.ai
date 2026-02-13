package world

import (
	"voxelcraft.ai/internal/protocol"
	eventspkg "voxelcraft.ai/internal/sim/world/feature/director/events"
	spawnspkg "voxelcraft.ai/internal/sim/world/feature/director/spawns"
	"voxelcraft.ai/internal/sim/world/logic/mathx"
)

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
