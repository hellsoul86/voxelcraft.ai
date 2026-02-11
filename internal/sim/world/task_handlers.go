package world

import (
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
)

type taskReqHandler func(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64)

var taskReqDispatch = map[string]taskReqHandler{
	TaskTypeStop:                     handleTaskStop,
	string(tasks.KindMoveTo):         handleTaskMoveTo,
	string(tasks.KindFollow):         handleTaskFollow,
	string(tasks.KindMine):           handleTaskMine,
	string(tasks.KindGather):         handleTaskGather,
	string(tasks.KindPlace):          handleTaskPlace,
	string(tasks.KindOpen):           handleTaskOpen,
	string(tasks.KindTransfer):       handleTaskTransfer,
	string(tasks.KindCraft):          handleTaskCraft,
	string(tasks.KindSmelt):          handleTaskSmelt,
	TaskTypeClaimLand:                handleTaskClaimLand,
	string(tasks.KindBuildBlueprint): handleTaskBuildBlueprint,
}

func handleTaskStop(_ *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	a.MoveTask = nil
	a.AddEvent(actionResult(nowTick, tr.ID, true, "", "stopped"))
}

func handleTaskMoveTo(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.MoveTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "movement task slot occupied"))
		return
	}
	// Reject targets outside the world boundary to avoid agents wandering into the "void"
	// (GetBlock returns AIR outside BoundaryR, and we don't generate chunks there).
	if !w.chunks.inBounds(Vec3i{X: tr.Target[0], Y: 0, Z: tr.Target[2]}) {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "out of bounds"))
		return
	}
	taskID := w.newTaskID()
	a.MoveTask = &tasks.MovementTask{
		TaskID:      taskID,
		Kind:        tasks.KindMoveTo,
		Target:      tasks.Vec3i{X: tr.Target[0], Y: 0, Z: tr.Target[2]},
		Tolerance:   tr.Tolerance,
		StartPos:    tasks.Vec3i{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{
		"t":       nowTick,
		"type":    "ACTION_RESULT",
		"ref":     tr.ID,
		"ok":      true,
		"task_id": taskID,
	})
}

func handleTaskFollow(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.MoveTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "movement task slot occupied"))
		return
	}
	if tr.TargetID == "" {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	dist := tr.Distance
	if dist <= 0 {
		dist = 2.0
	}
	if dist > 32 {
		dist = 32
	}
	target, ok := w.followTargetPos(tr.TargetID)
	if !ok {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "target not found"))
		return
	}
	taskID := w.newTaskID()
	a.MoveTask = &tasks.MovementTask{
		TaskID:      taskID,
		Kind:        tasks.KindFollow,
		Target:      v3ToTask(target),
		TargetID:    tr.TargetID,
		Distance:    dist,
		StartPos:    tasks.Vec3i{X: a.Pos.X, Y: a.Pos.Y, Z: a.Pos.Z},
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{
		"t":       nowTick,
		"type":    "ACTION_RESULT",
		"ref":     tr.ID,
		"ok":      true,
		"task_id": taskID,
	})
}

func handleTaskMine(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if !w.cfg.AllowMine {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_PERMISSION", "mining disabled in this world"))
		return
	}
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.BlockPos[1] != 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "2D world requires y==0"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindMine,
		BlockPos:    tasks.Vec3i{X: tr.BlockPos[0], Y: 0, Z: tr.BlockPos[2]},
		StartedTick: nowTick,
		WorkTicks:   0,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskGather(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.TargetID == "" {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	if w.items[tr.TargetID] == nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "item entity not found"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindGather,
		TargetID:    tr.TargetID,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskPlace(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if !w.cfg.AllowPlace {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_NO_PERMISSION", "placing disabled in this world"))
		return
	}
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.ItemID == "" {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing item_id"))
		return
	}
	if tr.BlockPos[1] != 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "2D world requires y==0"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindPlace,
		ItemID:      tr.ItemID,
		BlockPos:    tasks.Vec3i{X: tr.BlockPos[0], Y: 0, Z: tr.BlockPos[2]},
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskOpen(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.TargetID == "" {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindOpen,
		TargetID:    tr.TargetID,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskTransfer(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.Src == "" || tr.Dst == "" || tr.ItemID == "" || tr.Count <= 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing src/dst/item_id/count"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:       taskID,
		Kind:         tasks.KindTransfer,
		SrcContainer: tr.Src,
		DstContainer: tr.Dst,
		ItemID:       tr.ItemID,
		Count:        tr.Count,
		StartedTick:  nowTick,
		WorkTicks:    0,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskCraft(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.RecipeID == "" || tr.Count <= 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing recipe_id/count"))
		return
	}
	if _, ok := w.catalogs.Recipes.ByID[tr.RecipeID]; !ok {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "unknown recipe"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindCraft,
		RecipeID:    tr.RecipeID,
		Count:       tr.Count,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

func handleTaskSmelt(w *World, a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if a.WorkTask != nil {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.ItemID == "" || tr.Count <= 0 {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing item_id/count"))
		return
	}
	if _, ok := w.smeltByInput[tr.ItemID]; !ok {
		a.AddEvent(actionResult(nowTick, tr.ID, false, "E_INVALID_TARGET", "unsupported smelt item"))
		return
	}
	taskID := w.newTaskID()
	a.WorkTask = &tasks.WorkTask{
		TaskID:      taskID,
		Kind:        tasks.KindSmelt,
		ItemID:      tr.ItemID,
		Count:       tr.Count,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}

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
		if abs(anchor.X-c.Anchor.X) <= r+c.Radius && abs(anchor.Z-c.Anchor.Z) <= r+c.Radius {
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
	claimType := defaultClaimTypeForWorld(w.cfg.WorldType)
	due := uint64(0)
	if w.cfg.DayTicks > 0 {
		due = nowTick + uint64(w.cfg.DayTicks)
	}
	w.claims[landID] = &LandClaim{
		LandID:             landID,
		Owner:              a.ID,
		ClaimType:          claimType,
		Anchor:             anchor,
		Radius:             r,
		Flags:              defaultClaimFlags(claimType),
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
		Rotation:    normalizeRotation(tr.Rotation),
		BuildIndex:  0,
		StartedTick: nowTick,
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": tr.ID, "ok": true, "task_id": taskID})
}
