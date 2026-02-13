package instants

import (
	"voxelcraft.ai/internal/protocol"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type ClaimOwedEnv interface {
	GetContainerByID(id string) *modelpkg.Container
	Distance(a modelpkg.Vec3i, b modelpkg.Vec3i) int
}

type ActionResultFn func(tick uint64, ref string, ok bool, code string, message string) protocol.Event

func HandleClaimOwed(env ClaimOwedEnv, ar ActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.TerminalID == "" {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing terminal_id"))
		return
	}
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "contracts env unavailable"))
		return
	}
	c := env.GetContainerByID(inst.TerminalID)
	if c == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "terminal not found"))
		return
	}
	if env.Distance(a.Pos, c.Pos) > 3 {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
		return
	}
	owed := c.ClaimOwed(a.ID)
	if len(owed) == 0 {
		a.AddEvent(ar(nowTick, inst.ID, true, "", "nothing owed"))
		return
	}
	for item, n := range owed {
		if n > 0 {
			a.Inventory[item] += n
		}
	}
	a.AddEvent(ar(nowTick, inst.ID, true, "", "claimed"))
}
