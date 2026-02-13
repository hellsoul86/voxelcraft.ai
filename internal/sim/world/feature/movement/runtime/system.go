package runtime

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	detourpkg "voxelcraft.ai/internal/sim/world/feature/movement/detour"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type SystemInput struct {
	NowTick           uint64
	Weather           string
	ActiveEventID     string
	ActiveEventRadius int
	ActiveEventEnds   uint64
	ActiveEventCenter modelpkg.Vec3i
}

type MovementSystemEnv interface {
	SortedAgents() []*modelpkg.Agent
	FollowTargetPos(targetID string) (modelpkg.Vec3i, bool)
	SurfaceY(x int, z int) int
	BlockSolidAt(pos modelpkg.Vec3i) bool
	InBounds(pos modelpkg.Vec3i) bool

	LandAt(pos modelpkg.Vec3i) *modelpkg.LandClaim
	LandCoreContains(c *modelpkg.LandClaim, pos modelpkg.Vec3i) bool
	IsLandMember(agentID string, land *modelpkg.LandClaim) bool
	OrgByID(id string) *modelpkg.Organization
	TransferAccessTicket(ownerID string, item string, count int)
	RecordDenied(nowTick uint64)

	RecordStructureUsage(agentID string, pos modelpkg.Vec3i, nowTick uint64)
	OnBiome(a *modelpkg.Agent, nowTick uint64)
}

func RunMovementSystem(env MovementSystemEnv, in SystemInput) {
	if env == nil {
		return
	}
	for _, a := range env.SortedAgents() {
		mt := a.MoveTask
		if mt == nil {
			continue
		}

		var target modelpkg.Vec3i
		switch mt.Kind {
		case tasks.KindMoveTo:
			target = modelpkg.Vec3i{X: mt.Target.X, Y: mt.Target.Y, Z: mt.Target.Z}
			want := MoveTolerance(mt.Tolerance)
			if DistXZ(
				Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
				Pos{X: target.X, Y: target.Y, Z: target.Z},
			) <= want {
				env.RecordStructureUsage(a.ID, a.Pos, in.NowTick)
				env.OnBiome(a, in.NowTick)
				a.MoveTask = nil
				a.AddEvent(protocol.Event{"t": in.NowTick, "type": "TASK_DONE", "task_id": mt.TaskID, "kind": string(mt.Kind)})
				continue
			}
		case tasks.KindFollow:
			t, ok := env.FollowTargetPos(mt.TargetID)
			if !ok {
				a.MoveTask = nil
				a.AddEvent(protocol.Event{"t": in.NowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_INVALID_TARGET", "message": "follow target not found"})
				continue
			}
			mt.Target = tasks.Vec3i{X: t.X, Y: t.Y, Z: t.Z}
			target = t
			want := MoveTolerance(mt.Distance)
			if DistXZ(
				Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
				Pos{X: target.X, Y: target.Y, Z: target.Z},
			) <= want {
				continue
			}
		default:
			continue
		}

		if ShouldSkipStorm(in.Weather, in.NowTick) {
			continue
		}
		if ShouldSkipFlood(
			in.ActiveEventID,
			in.ActiveEventRadius,
			in.NowTick,
			in.ActiveEventEnds,
			Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
			Pos{X: in.ActiveEventCenter.X, Y: in.ActiveEventCenter.Y, Z: in.ActiveEventCenter.Z},
		) {
			continue
		}

		const moveCost = 8
		if a.StaminaMilli < moveCost {
			continue
		}
		a.StaminaMilli -= moveCost

		dx := target.X - a.Pos.X
		dz := target.Z - a.Pos.Z
		primaryX := PrimaryAxis(dx, dz)

		next := PrimaryStep(Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z}, dx, dz, primaryX)
		next.Y = env.SurfaceY(next.X, next.Z)

		if env.BlockSolidAt(modelpkg.Vec3i{X: next.X, Y: next.Y, Z: next.Z}) {
			alt := SecondaryStep(Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z}, dx, dz, primaryX)
			if alt != (Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z}) {
				alt.Y = env.SurfaceY(alt.X, alt.Z)
				if !env.BlockSolidAt(modelpkg.Vec3i{X: alt.X, Y: alt.Y, Z: alt.Z}) {
					next = alt
				}
			}
		}

		if env.BlockSolidAt(modelpkg.Vec3i{X: next.X, Y: next.Y, Z: next.Z}) {
			if alt, ok := detourpkg.DetourStep2D(
				detourpkg.Pos{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
				detourpkg.Pos{X: target.X, Y: target.Y, Z: target.Z},
				16,
				func(p detourpkg.Pos) bool {
					return env.InBounds(modelpkg.Vec3i{X: p.X, Y: p.Y, Z: p.Z})
				},
				func(p detourpkg.Pos) bool {
					return env.BlockSolidAt(modelpkg.Vec3i{X: p.X, Y: p.Y, Z: p.Z})
				},
			); ok {
				next = Pos{X: alt.X, Y: alt.Y, Z: alt.Z}
			}
		}

		nextPos := modelpkg.Vec3i{X: next.X, Y: next.Y, Z: next.Z}

		if toLand := env.LandAt(nextPos); toLand != nil && env.LandCoreContains(toLand, nextPos) && !env.IsLandMember(a.ID, toLand) {
			if org := env.OrgByID(toLand.Owner); org != nil && org.Kind == modelpkg.OrgCity {
				fromLand := env.LandAt(a.Pos)
				entering := fromLand == nil || fromLand.LandID != toLand.LandID || !env.LandCoreContains(toLand, a.Pos)
				if entering && a.RepLaw > 0 && a.RepLaw < 200 {
					a.MoveTask = nil
					env.RecordDenied(in.NowTick)
					a.AddEvent(protocol.Event{"t": in.NowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_NO_PERMISSION", "message": "wanted: law reputation too low"})
					continue
				}
			}
		}

		if toLand := env.LandAt(nextPos); toLand != nil && toLand.AccessPassEnabled && env.LandCoreContains(toLand, nextPos) && !env.IsLandMember(a.ID, toLand) {
			fromLand := env.LandAt(a.Pos)
			entering := fromLand == nil || fromLand.LandID != toLand.LandID || !env.LandCoreContains(toLand, a.Pos)
			if entering {
				item := strings.TrimSpace(toLand.AccessTicketItem)
				cost := toLand.AccessTicketCost
				if item == "" || cost <= 0 {
					a.MoveTask = nil
					env.RecordDenied(in.NowTick)
					a.AddEvent(protocol.Event{"t": in.NowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_NO_PERMISSION", "message": "access pass required"})
					continue
				}
				if a.Inventory[item] < cost {
					a.MoveTask = nil
					env.RecordDenied(in.NowTick)
					a.AddEvent(protocol.Event{"t": in.NowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_NO_RESOURCE", "message": "need access ticket"})
					continue
				}
				a.Inventory[item] -= cost
				env.TransferAccessTicket(toLand.Owner, item, cost)
				a.AddEvent(protocol.Event{"t": in.NowTick, "type": "ACCESS_PASS", "land_id": toLand.LandID, "item": item, "count": cost})
			}
		}

		if env.BlockSolidAt(nextPos) {
			a.MoveTask = nil
			a.AddEvent(protocol.Event{"t": in.NowTick, "type": "TASK_FAIL", "task_id": mt.TaskID, "code": "E_BLOCKED", "message": "blocked"})
			continue
		}

		a.Pos = nextPos
		env.RecordStructureUsage(a.ID, a.Pos, in.NowTick)
		env.OnBiome(a, in.NowTick)
	}
}
