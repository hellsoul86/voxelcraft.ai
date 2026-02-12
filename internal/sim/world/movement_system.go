package world

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	detourpkg "voxelcraft.ai/internal/sim/world/feature/movement/detour"
	runtimepkg "voxelcraft.ai/internal/sim/world/feature/movement/runtime"
)

func (w *World) systemMovementImpl(nowTick uint64) {
	for _, a := range w.sortedAgents() {
		mt := a.MoveTask
		if mt == nil {
			continue
		}
		var target Vec3i
		switch mt.Kind {
		case tasks.KindMoveTo:
			target = v3FromTask(mt.Target)
			want := runtimepkg.MoveTolerance(mt.Tolerance)
			// Complete when within tolerance; do not teleport to the exact target to avoid skipping obstacles.
			if distXZ(a.Pos, target) <= want {
				w.recordStructureUsage(a.ID, a.Pos, nowTick)
				w.funOnBiome(a, nowTick)
				a.MoveTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": mt.TaskID, "kind": string(mt.Kind)})
				continue
			}

		case tasks.KindFollow:
			t, ok := w.followTargetPos(mt.TargetID)
			if !ok {
				a.MoveTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_INVALID_TARGET", "message": "follow target not found"})
				continue
			}
			mt.Target = v3ToTask(t)
			target = t

			want := runtimepkg.MoveTolerance(mt.Distance)
			if distXZ(a.Pos, target) <= want {
				// Stay close; keep task active until canceled.
				continue
			}

		default:
			continue
		}

		// Storm slows travel but should not deadlock tasks.
		if runtimepkg.ShouldSkipStorm(w.weather, nowTick) {
			continue
		}

		// Event hazard: flood zones slow travel.
		if runtimepkg.ShouldSkipFlood(
			w.activeEventID,
			w.activeEventRadius,
			nowTick,
			w.activeEventEnds,
			runtimepkg.Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
			runtimepkg.Pos{X: w.activeEventCenter.X, Y: w.activeEventCenter.Y, Z: w.activeEventCenter.Z},
		) {
			continue
		}

		// Moving costs stamina; if too tired, wait and recover.
		const moveCost = 8
		if a.StaminaMilli < moveCost {
			continue
		}
		a.StaminaMilli -= moveCost

		// Deterministic 2D stepping with minimal obstacle avoidance:
		// - Pick primary axis by abs(dx)>=abs(dz)
		// - If the next cell on the primary axis is blocked by a solid block, try the secondary axis.
		dx := target.X - a.Pos.X
		dz := target.Z - a.Pos.Z

		primaryX := runtimepkg.PrimaryAxis(dx, dz)
		next := a.Pos
		p1 := runtimepkg.PrimaryStep(runtimepkg.Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z}, dx, dz, primaryX)
		next1 := Vec3i{X: p1.X, Y: p1.Y, Z: p1.Z}
		next1.Y = w.surfaceY(next1.X, next1.Z)
		next = next1

		if w.blockSolid(w.chunks.GetBlock(next1)) {
			// Try the secondary axis only when primary step is blocked.
			p2 := runtimepkg.SecondaryStep(runtimepkg.Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z}, dx, dz, primaryX)
			next2 := Vec3i{X: p2.X, Y: p2.Y, Z: p2.Z}
			if next2 != a.Pos {
				next2.Y = w.surfaceY(next2.X, next2.Z)
				if !w.blockSolid(w.chunks.GetBlock(next2)) {
					next = next2
				}
			}
		}

		// If both primary+secondary are blocked, attempt a small deterministic detour so agents
		// don't have to constantly re-issue MOVE_TO on cluttered terrain.
		if w.blockSolid(w.chunks.GetBlock(next)) {
			if alt, ok := detourpkg.DetourStep2D(
				detourpkg.Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
				detourpkg.Pos{X: target.X, Y: target.Y, Z: target.Z},
				16,
				func(p detourpkg.Pos) bool {
					return w.chunks.inBounds(Vec3i{X: p.X, Y: p.Y, Z: p.Z})
				},
				func(p detourpkg.Pos) bool {
					return w.blockSolid(w.chunks.GetBlock(Vec3i{X: p.X, Y: p.Y, Z: p.Z}))
				},
			); ok {
				next = Vec3i{X: alt.X, Y: alt.Y, Z: alt.Z}
			}
		}

		// Reputation consequence: low Law rep agents may be blocked from entering a CITY core area.
		// This is a system-level "wanted" restriction separate from access passes.
		if toLand := w.landAt(next); toLand != nil && w.landCoreContains(toLand, next) && !w.isLandMember(a.ID, toLand) {
			if org := w.orgByID(toLand.Owner); org != nil && org.Kind == OrgCity {
				fromLand := w.landAt(a.Pos)
				entering := fromLand == nil || fromLand.LandID != toLand.LandID || !w.landCoreContains(toLand, a.Pos)
				if entering && a.RepLaw > 0 && a.RepLaw < 200 {
					a.MoveTask = nil
					if w.stats != nil {
						w.stats.RecordDenied(nowTick)
					}
					a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_NO_PERMISSION", "message": "wanted: law reputation too low"})
					continue
				}
			}
		}

		// Land core access pass (law): charge ticket on core entry for non-members.
		if toLand := w.landAt(next); toLand != nil && toLand.AccessPassEnabled && w.landCoreContains(toLand, next) && !w.isLandMember(a.ID, toLand) {
			fromLand := w.landAt(a.Pos)
			entering := fromLand == nil || fromLand.LandID != toLand.LandID || !w.landCoreContains(toLand, a.Pos)
			if entering {
				item := strings.TrimSpace(toLand.AccessTicketItem)
				cost := toLand.AccessTicketCost
				if item == "" || cost <= 0 {
					// Misconfigured law: treat as blocked.
					a.MoveTask = nil
					if w.stats != nil {
						w.stats.RecordDenied(nowTick)
					}
					a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_NO_PERMISSION", "message": "access pass required"})
					continue
				}
				if a.Inventory[item] < cost {
					a.MoveTask = nil
					if w.stats != nil {
						w.stats.RecordDenied(nowTick)
					}
					a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_NO_RESOURCE", "message": "need access ticket"})
					continue
				}
				a.Inventory[item] -= cost
				// Credit to land owner if present (agent or org treasury); else burn.
				if toLand.Owner != "" {
					if owner := w.agents[toLand.Owner]; owner != nil {
						owner.Inventory[item] += cost
					} else if org := w.orgByID(toLand.Owner); org != nil {
						w.orgTreasury(org)[item] += cost
					}
				}
				a.AddEvent(protocol.Event{"t": nowTick, "type": "ACCESS_PASS", "land_id": toLand.LandID, "item": item, "count": cost})
			}
		}

		// Basic collision: treat solid blocks as blocking; allow non-solid (e.g. water/torch/wire).
		if w.blockSolid(w.chunks.GetBlock(Vec3i{X: next.X, Y: next.Y, Z: next.Z})) {
			a.MoveTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_BLOCKED", "message": "blocked"})
			continue
		}
		a.Pos = next
		w.recordStructureUsage(a.ID, a.Pos, nowTick)
		w.funOnBiome(a, nowTick)
	}
}
