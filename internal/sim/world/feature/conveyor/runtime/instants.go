package runtime

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type ActionResultFn func(tick uint64, ref string, ok bool, code string, message string) protocol.Event

type SwitchInstantEnv interface {
	ParseContainerID(id string) (typ string, pos modelpkg.Vec3i, ok bool)
	BlockNameAt(pos modelpkg.Vec3i) string
	Distance(a modelpkg.Vec3i, b modelpkg.Vec3i) int
	CanBuildAt(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool
	SwitchStateAt(pos modelpkg.Vec3i) bool
	SetSwitchState(pos modelpkg.Vec3i, on bool)
	SwitchIDAt(pos modelpkg.Vec3i) string
	AuditSwitchToggle(nowTick uint64, actorID string, pos modelpkg.Vec3i, switchID string, on bool)
	BumpLawRep(agentID string, delta int)
	RecordDenied(nowTick uint64)
}

func HandleToggleSwitch(env SwitchInstantEnv, ar ActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "switch env unavailable"))
		return
	}
	target := strings.TrimSpace(inst.TargetID)
	if target == "" {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	typ, pos, ok := env.ParseContainerID(target)
	if !ok || typ != "SWITCH" {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BAD_REQUEST", "invalid switch target"))
		return
	}
	if env.BlockNameAt(pos) != "SWITCH" {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "switch not found"))
		return
	}
	if env.Distance(a.Pos, pos) > 3 {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
		return
	}
	if !env.CanBuildAt(a.ID, pos, nowTick) {
		env.BumpLawRep(a.ID, -1)
		env.RecordDenied(nowTick)
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_PERMISSION", "switch toggle denied"))
		return
	}
	on := !env.SwitchStateAt(pos)
	env.SetSwitchState(pos, on)
	switchID := env.SwitchIDAt(pos)
	env.AuditSwitchToggle(nowTick, a.ID, pos, switchID, on)
	a.AddEvent(protocol.Event{"t": nowTick, "type": "SWITCH", "switch_id": switchID, "pos": pos.ToArray(), "on": on})
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}
