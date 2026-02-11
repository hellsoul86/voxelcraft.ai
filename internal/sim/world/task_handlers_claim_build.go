package world

import (
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	"voxelcraft.ai/internal/sim/world/feature/governance"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
	"voxelcraft.ai/internal/sim/world/logic/mathx"
)

func handleTaskClaimLand(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if !w.cfg.AllowClaims {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_PERMISSION", "claims disabled in this world"))
		return
	}
	r := tr.Radius
	if r <= 0 {
		r = 32
	}
	if r > 128 {
		r = 128
	}
	if tr.Anchor[1] != 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "2D world requires y==0"))
		return
	}
	anchor := Vec3i{X: tr.Anchor[0], Y: tr.Anchor[1], Z: tr.Anchor[2]}
	if !w.chunks.inBounds(anchor) {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "out of bounds"))
		return
	}
	// Must be allowed to build at anchor (unclaimed or owned land with build permission).
	if !w.canBuildAt(a.ID, anchor, nowTick) {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_PERMISSION", "cannot claim here"))
		return
	}
	// Must have resources: 1 battery + 1 crystal shard (MVP).
	if a.Inventory["BATTERY"] < 1 || a.Inventory["CRYSTAL_SHARD"] < 1 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_RESOURCE", "need BATTERY + CRYSTAL_SHARD"))
		return
	}
	// Must not overlap existing claims.
	for _, c := range w.claims {
		// Conservative overlap check: if anchors are close enough, treat as overlap.
		if mathx.AbsInt(anchor.X-c.Anchor.X) <= r+c.Radius && mathx.AbsInt(anchor.Z-c.Anchor.Z) <= r+c.Radius {
			a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "claim overlaps existing land"))
			return
		}
	}
	// Place Claim Totem block at anchor.
	if w.chunks.GetBlock(anchor) != w.chunks.gen.Air {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BLOCKED", "anchor occupied"))
		return
	}
	totemID, ok := w.catalogs.Blocks.Index["CLAIM_TOTEM"]
	if !ok {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INTERNAL", "missing CLAIM_TOTEM block"))
		return
	}

	// Consume cost.
	a.Inventory["BATTERY"]--
	a.Inventory["CRYSTAL_SHARD"]--

	w.chunks.SetBlock(anchor, totemID)
	w.auditSetBlock(nowTick, a.ID, anchor, w.chunks.gen.Air, totemID, "CLAIM_LAND")

	landID := w.newLandID(a.ID)
	claimType := governance.DefaultClaimTypeForWorld(w.cfg.WorldType)
	baseFlags := governance.DefaultClaimFlags(claimType)
	due := uint64(0)
	if w.cfg.DayTicks > 0 {
		due = nowTick + uint64(w.cfg.DayTicks)
	}
	w.claims[landID] = &LandClaim{
		LandID:    landID,
		Owner:     a.ID,
		ClaimType: claimType,
		Anchor:    anchor,
		Radius:    r,
		Flags: ClaimFlags{
			AllowBuild:  baseFlags.AllowBuild,
			AllowBreak:  baseFlags.AllowBreak,
			AllowDamage: baseFlags.AllowDamage,
			AllowTrade:  baseFlags.AllowTrade,
		},
		MaintenanceDueTick: due,
		MaintenanceStage:   0,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "land_id": landID})
}

func handleTaskBuildBlueprint(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if !w.cfg.AllowBuild {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_PERMISSION", "blueprint build disabled in this world"))
		return
	}
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.BlueprintID == "" {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing blueprint_id"))
		return
	}
	if tr.Anchor[1] != 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "2D world requires y==0"))
		return
	}
	if _, ok := w.catalogs.Blueprints.ByID[tr.BlueprintID]; !ok {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "unknown blueprint"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindBuildBlueprint,
		BlueprintID: tr.BlueprintID,
		Anchor:      tasks.Vec3i{X: tr.Anchor[0], Y: 0, Z: tr.Anchor[2]},
		Rotation:    blueprint.NormalizeRotation(tr.Rotation),
		BuildIndex:  0,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}
