package world

type conveyorInstantsWorldEnv struct {
	w *World
}

func (e conveyorInstantsWorldEnv) ParseContainerID(id string) (typ string, pos Vec3i, ok bool) {
	if e.w == nil {
		return "", Vec3i{}, false
	}
	return parseContainerID(id)
}

func (e conveyorInstantsWorldEnv) BlockNameAt(pos Vec3i) string {
	if e.w == nil {
		return ""
	}
	return e.w.blockName(e.w.chunks.GetBlock(pos))
}

func (e conveyorInstantsWorldEnv) Distance(a Vec3i, b Vec3i) int {
	return Manhattan(a, b)
}

func (e conveyorInstantsWorldEnv) CanBuildAt(agentID string, pos Vec3i, nowTick uint64) bool {
	if e.w == nil {
		return false
	}
	return e.w.canBuildAt(agentID, pos, nowTick)
}

func (e conveyorInstantsWorldEnv) SwitchStateAt(pos Vec3i) bool {
	if e.w == nil {
		return false
	}
	if e.w.switches == nil {
		return false
	}
	return e.w.switches[pos]
}

func (e conveyorInstantsWorldEnv) SetSwitchState(pos Vec3i, on bool) {
	if e.w == nil {
		return
	}
	if e.w.switches == nil {
		e.w.switches = map[Vec3i]bool{}
	}
	e.w.switches[pos] = on
}

func (e conveyorInstantsWorldEnv) SwitchIDAt(pos Vec3i) string {
	return switchIDAt(pos)
}

func (e conveyorInstantsWorldEnv) AuditSwitchToggle(nowTick uint64, actorID string, pos Vec3i, switchID string, on bool) {
	if e.w == nil {
		return
	}
	e.w.auditEvent(nowTick, actorID, "SWITCH_TOGGLE", pos, "TOGGLE_SWITCH", map[string]any{
		"switch_id": switchID,
		"on":        on,
	})
}

func (e conveyorInstantsWorldEnv) BumpLawRep(agentID string, delta int) {
	if e.w == nil {
		return
	}
	e.w.bumpRepLaw(agentID, delta)
}

func (e conveyorInstantsWorldEnv) RecordDenied(nowTick uint64) {
	if e.w == nil || e.w.stats == nil {
		return
	}
	e.w.stats.RecordDenied(nowTick)
}
