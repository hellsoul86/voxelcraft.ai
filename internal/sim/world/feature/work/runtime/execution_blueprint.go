package runtime

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/tasks"
	limitspkg "voxelcraft.ai/internal/sim/world/feature/work/limits"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
)

type WorkExecBlueprintEnv interface {
	GetBlueprint(id string) (catalogs.BlueprintDef, bool)
	InBounds(pos modelpkg.Vec3i) bool
	BlockIDByName(blockName string) (uint16, bool)
	CanBuildAt(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool
	RecordDenied(nowTick uint64)
	BumpRepLaw(agentID string, delta int)
	BlockAt(pos modelpkg.Vec3i) uint16
	AirBlockID() uint16
	EnsureBlueprintMaterials(a *modelpkg.Agent, anchor modelpkg.Vec3i, needCost []catalogs.ItemCount, nowTick uint64) (bool, string)
	SetBlock(pos modelpkg.Vec3i, blockID uint16)
	AuditSetBlock(nowTick uint64, actor string, pos modelpkg.Vec3i, from uint16, to uint16, reason string)
	EnsureContainerForPlacedBlock(pos modelpkg.Vec3i, blockName string)
}

type BlueprintTickResult struct {
	Completed bool
	NoopDone  bool
	Anchor    modelpkg.Vec3i
	Rotation  int
}

func TickBuildBlueprint(env WorkExecBlueprintEnv, a *modelpkg.Agent, wt *tasks.WorkTask, nowTick uint64, blocksPerTick int) BlueprintTickResult {
	if env == nil || a == nil || wt == nil {
		return BlueprintTickResult{}
	}
	bp, ok := env.GetBlueprint(wt.BlueprintID)
	if !ok {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "unknown blueprint"})
		return BlueprintTickResult{}
	}
	anchor := modelpkg.Vec3i{X: wt.Anchor.X, Y: wt.Anchor.Y, Z: wt.Anchor.Z}
	rot := blueprint.NormalizeRotation(wt.Rotation)

	fail := func(code, msg string, deniedLaw bool) BlueprintTickResult {
		a.WorkTask = nil
		if deniedLaw {
			env.BumpRepLaw(a.ID, -1)
			env.RecordDenied(nowTick)
		}
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
		return BlueprintTickResult{}
	}

	// On first tick, validate and charge remaining material cost (resume-safe).
	if wt.BuildIndex == 0 && wt.WorkTicks == 0 {
		alreadyCorrect := map[string]int{}
		correct := 0
		for _, p := range bp.Blocks {
			off := blueprint.RotateOffset(p.Pos, rot)
			pos := modelpkg.Vec3i{X: anchor.X + off[0], Y: anchor.Y + off[1], Z: anchor.Z + off[2]}
			if !env.InBounds(pos) {
				return fail("E_INVALID_TARGET", "out of bounds", false)
			}
			bid, ok := env.BlockIDByName(p.Block)
			if !ok {
				return fail("E_INTERNAL", "unknown block in blueprint", false)
			}
			if !env.CanBuildAt(a.ID, pos, nowTick) {
				return fail("E_NO_PERMISSION", "build denied", true)
			}
			cur := env.BlockAt(pos)
			if cur != env.AirBlockID() {
				if cur == bid {
					alreadyCorrect[p.Block]++
					correct++
					continue
				}
				return fail("E_BLOCKED", "space occupied", false)
			}
		}

		// Anti-exploit: already-complete blueprint becomes a no-op TASK_DONE.
		if blueprint.FullySatisfied(correct, len(bp.Blocks)) {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
			return BlueprintTickResult{NoopDone: true, Anchor: anchor, Rotation: rot}
		}

		baseCost := make([]blueprint.ItemCount, 0, len(bp.Cost))
		for _, c := range bp.Cost {
			if strings.TrimSpace(c.Item) == "" || c.Count <= 0 {
				continue
			}
			baseCost = append(baseCost, blueprint.ItemCount{Item: c.Item, Count: c.Count})
		}
		remaining := blueprint.RemainingCost(baseCost, alreadyCorrect)
		needCost := make([]catalogs.ItemCount, 0, len(remaining))
		for _, c := range remaining {
			needCost = append(needCost, catalogs.ItemCount{Item: c.Item, Count: c.Count})
		}

		for _, c := range needCost {
			if a.Inventory[c.Item] >= c.Count {
				continue
			}
			okPull, msg := env.EnsureBlueprintMaterials(a, anchor, needCost, nowTick)
			if !okPull {
				a.WorkTask = nil
				if msg == "" {
					msg = "missing materials"
				}
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": msg})
				return BlueprintTickResult{}
			}
			break
		}
		for _, c := range needCost {
			a.Inventory[c.Item] -= c.Count
		}
	}

	placed := 0
	limit := limitspkg.ClampBlocksPerTick(blocksPerTick)
	for placed < limit && wt.BuildIndex < len(bp.Blocks) {
		p := bp.Blocks[wt.BuildIndex]
		off := blueprint.RotateOffset(p.Pos, rot)
		pos := modelpkg.Vec3i{X: anchor.X + off[0], Y: anchor.Y + off[1], Z: anchor.Z + off[2]}
		if !env.InBounds(pos) {
			return fail("E_INVALID_TARGET", "out of bounds", false)
		}
		bid, ok := env.BlockIDByName(p.Block)
		if !ok {
			return fail("E_INTERNAL", "unknown block in blueprint", false)
		}
		if !env.CanBuildAt(a.ID, pos, nowTick) {
			return fail("E_NO_PERMISSION", "build denied", true)
		}
		cur := env.BlockAt(pos)
		if cur != env.AirBlockID() {
			if cur == bid {
				wt.BuildIndex++
				continue
			}
			return fail("E_BLOCKED", "space occupied", false)
		}
		env.SetBlock(pos, bid)
		env.AuditSetBlock(nowTick, a.ID, pos, env.AirBlockID(), bid, "BUILD_BLUEPRINT")
		env.EnsureContainerForPlacedBlock(pos, p.Block)

		wt.BuildIndex++
		placed++
	}

	if wt.BuildIndex >= len(bp.Blocks) {
		return BlueprintTickResult{
			Completed: true,
			Anchor:    anchor,
			Rotation:  rot,
		}
	}
	return BlueprintTickResult{}
}
