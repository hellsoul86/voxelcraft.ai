package runtime

import (
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
	"voxelcraft.ai/internal/sim/world/logic/blueprint"
)

type BlueprintRequestEnv interface {
	NewTaskID() string
	BlueprintExists(blueprintID string) bool
}

func HandleTaskBuildBlueprint(env BlueprintRequestEnv, ar ActionResultFn, a *modelpkg.Agent, tr protocol.TaskReq, nowTick uint64, allowBuild bool) {
	if !allowBuild {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_NO_PERMISSION", "blueprint build disabled in this world"))
		return
	}
	if a.WorkTask != nil {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_CONFLICT", "work task slot occupied"))
		return
	}
	if tr.BlueprintID == "" {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_BAD_REQUEST", "missing blueprint_id"))
		return
	}
	if tr.Anchor[1] != 0 {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_INVALID_TARGET", "2D world requires y==0"))
		return
	}
	if env == nil || !env.BlueprintExists(tr.BlueprintID) {
		a.AddEvent(ar(nowTick, tr.ID, false, "E_INVALID_TARGET", "unknown blueprint"))
		return
	}
	taskID := env.NewTaskID()
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
