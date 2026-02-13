package conveyor

import modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"

type Env struct {
	ParseContainerIDFn  func(id string) (typ string, pos modelpkg.Vec3i, ok bool)
	BlockNameAtFn       func(pos modelpkg.Vec3i) string
	DistanceFn          func(a modelpkg.Vec3i, b modelpkg.Vec3i) int
	CanBuildAtFn        func(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool
	SwitchStateAtFn     func(pos modelpkg.Vec3i) bool
	SetSwitchStateFn    func(pos modelpkg.Vec3i, on bool)
	SwitchIDAtFn        func(pos modelpkg.Vec3i) string
	AuditSwitchToggleFn func(nowTick uint64, actorID string, pos modelpkg.Vec3i, switchID string, on bool)
	BumpLawRepFn        func(agentID string, delta int)
	RecordDeniedFn      func(nowTick uint64)
}

func (e Env) ParseContainerID(id string) (typ string, pos modelpkg.Vec3i, ok bool) {
	if e.ParseContainerIDFn == nil {
		return "", modelpkg.Vec3i{}, false
	}
	return e.ParseContainerIDFn(id)
}

func (e Env) BlockNameAt(pos modelpkg.Vec3i) string {
	if e.BlockNameAtFn == nil {
		return ""
	}
	return e.BlockNameAtFn(pos)
}

func (e Env) Distance(a modelpkg.Vec3i, b modelpkg.Vec3i) int {
	if e.DistanceFn == nil {
		return 0
	}
	return e.DistanceFn(a, b)
}

func (e Env) CanBuildAt(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool {
	if e.CanBuildAtFn == nil {
		return false
	}
	return e.CanBuildAtFn(agentID, pos, nowTick)
}

func (e Env) SwitchStateAt(pos modelpkg.Vec3i) bool {
	if e.SwitchStateAtFn == nil {
		return false
	}
	return e.SwitchStateAtFn(pos)
}

func (e Env) SetSwitchState(pos modelpkg.Vec3i, on bool) {
	if e.SetSwitchStateFn != nil {
		e.SetSwitchStateFn(pos, on)
	}
}

func (e Env) SwitchIDAt(pos modelpkg.Vec3i) string {
	if e.SwitchIDAtFn == nil {
		return ""
	}
	return e.SwitchIDAtFn(pos)
}

func (e Env) AuditSwitchToggle(nowTick uint64, actorID string, pos modelpkg.Vec3i, switchID string, on bool) {
	if e.AuditSwitchToggleFn != nil {
		e.AuditSwitchToggleFn(nowTick, actorID, pos, switchID, on)
	}
}

func (e Env) BumpLawRep(agentID string, delta int) {
	if e.BumpLawRepFn != nil {
		e.BumpLawRepFn(agentID, delta)
	}
}

func (e Env) RecordDenied(nowTick uint64) {
	if e.RecordDeniedFn != nil {
		e.RecordDeniedFn(nowTick)
	}
}
