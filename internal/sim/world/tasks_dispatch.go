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
