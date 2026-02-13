package world

import (
	entitiesruntimepkg "voxelcraft.ai/internal/sim/world/feature/entities/runtime"
	permissionspkg "voxelcraft.ai/internal/sim/world/feature/governance/permissions"
)

func containerID(typ string, pos Vec3i) string {
	return entitiesruntimepkg.ContainerID(typ, pos)
}

func parseContainerID(id string) (typ string, pos Vec3i, ok bool) {
	return entitiesruntimepkg.ParseContainerID(id)
}

func (w *World) ensureContainerForPlacedBlock(pos Vec3i, blockName string) {
	e := entitiesruntimepkg.EffectsForPlacedBlock(blockName)
	if e.ContainerType != "" {
		w.ensureContainer(pos, e.ContainerType)
	}
	if e.EnsureBoard {
		w.ensureBoard(pos)
	}
	if e.EnsureSign {
		w.ensureSign(pos)
	}
	if e.EnsureConveyor {
		w.ensureConveyor(pos, e.ConveyorDX, e.ConveyorDZ)
	}
	if e.EnsureSwitch {
		w.ensureSwitch(pos, e.SwitchOn)
	}
}

func (w *World) ensureContainer(pos Vec3i, typ string) *Container {
	return entitiesruntimepkg.EnsureContainer(w.containers, pos, typ)
}

func (w *World) removeContainer(pos Vec3i) *Container {
	return entitiesruntimepkg.RemoveContainer(w.containers, pos)
}

func (w *World) getContainerByID(id string) *Container {
	return entitiesruntimepkg.GetContainerByID(w.containers, id)
}

func (w *World) canWithdrawFromContainer(agentID string, pos Vec3i) bool {
	land := w.landAt(pos)
	if land == nil {
		return permissionspkg.CanWithdrawContainer(false, false, 0)
	}
	return permissionspkg.CanWithdrawContainer(true, w.isLandMember(agentID, land), land.MaintenanceStage)
}

// --- Block runtime meta: SIGN / CONVEYOR / SWITCH ---

func signIDAt(pos Vec3i) string { return entitiesruntimepkg.SignIDAt(pos) }

func (w *World) ensureSign(pos Vec3i) *Sign {
	return entitiesruntimepkg.EnsureSign(w.signs, pos)
}

func (w *World) removeSign(nowTick uint64, actor string, pos Vec3i, reason string) {
	if !entitiesruntimepkg.RemoveSign(w.signs, pos) {
		return
	}
	// Record the removal as a separate audit event (the SET_BLOCK audit already exists too).
	w.auditEvent(nowTick, actor, "SIGN_REMOVE", pos, reason, map[string]any{
		"sign_id": signIDAt(pos),
	})
}

func (w *World) sortedSignPositionsNear(pos Vec3i, dist int) []Vec3i {
	return entitiesruntimepkg.SortedSignPositionsNear(w.signs, pos, dist)
}

func conveyorIDAt(pos Vec3i) string { return entitiesruntimepkg.ConveyorIDAt(pos) }

func (w *World) ensureConveyor(pos Vec3i, dx, dz int) {
	entitiesruntimepkg.EnsureConveyor(w.conveyors, pos, dx, dz)
}

func (w *World) removeConveyor(nowTick uint64, actor string, pos Vec3i, reason string) {
	if !entitiesruntimepkg.RemoveConveyor(w.conveyors, pos) {
		return
	}
	w.auditEvent(nowTick, actor, "CONVEYOR_REMOVE", pos, reason, map[string]any{
		"conveyor_id": conveyorIDAt(pos),
	})
}

func (w *World) sortedConveyorPositionsNear(pos Vec3i, dist int) []Vec3i {
	return entitiesruntimepkg.SortedConveyorPositionsNear(w.conveyors, pos, dist, func(p Vec3i) string {
		return w.blockName(w.chunks.GetBlock(p))
	})
}

func switchIDAt(pos Vec3i) string { return entitiesruntimepkg.SwitchIDAt(pos) }

func (w *World) ensureSwitch(pos Vec3i, on bool) {
	if w.switches == nil {
		w.switches = map[Vec3i]bool{}
	}
	entitiesruntimepkg.EnsureSwitch(w.switches, pos, on)
}

func (w *World) removeSwitch(nowTick uint64, actor string, pos Vec3i, reason string) {
	if w.switches == nil || !entitiesruntimepkg.RemoveSwitch(w.switches, pos) {
		return
	}
	w.auditEvent(nowTick, actor, "SWITCH_REMOVE", pos, reason, map[string]any{
		"switch_id": switchIDAt(pos),
	})
}

func (w *World) sortedSwitchPositionsNear(pos Vec3i, dist int) []Vec3i {
	return entitiesruntimepkg.SortedSwitchPositionsNear(w.switches, pos, dist, func(p Vec3i) string {
		return w.blockName(w.chunks.GetBlock(p))
	})
}
