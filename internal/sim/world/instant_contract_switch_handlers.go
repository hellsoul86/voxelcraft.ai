package world

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
)

func handleInstantToggleSwitch(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	target := strings.TrimSpace(inst.TargetID)
	if target == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	typ, pos, ok := parseContainerID(target)
	if !ok || typ != "SWITCH" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "invalid switch target"))
		return
	}
	if w.blockName(w.chunks.GetBlock(pos)) != "SWITCH" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "switch not found"))
		return
	}
	if Manhattan(a.Pos, pos) > 3 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
		return
	}
	if !w.canBuildAt(a.ID, pos, nowTick) {
		w.bumpRepLaw(a.ID, -1)
		if w.stats != nil {
			w.stats.RecordDenied(nowTick)
		}
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "switch toggle denied"))
		return
	}
	if w.switches == nil {
		w.switches = map[Vec3i]bool{}
	}
	on := !w.switches[pos]
	w.switches[pos] = on
	w.auditEvent(nowTick, a.ID, "SWITCH_TOGGLE", pos, "TOGGLE_SWITCH", map[string]any{
		"switch_id": switchIDAt(pos),
		"on":        on,
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "SWITCH", "switch_id": switchIDAt(pos), "pos": pos.ToArray(), "on": on})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantClaimOwed(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	// Claim owed items from a terminal container to self.
	if inst.TerminalID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing terminal_id"))
		return
	}
	c := w.getContainerByID(inst.TerminalID)
	if c == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "terminal not found"))
		return
	}
	if Manhattan(a.Pos, c.Pos) > 3 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
		return
	}
	owed := c.claimOwed(a.ID)
	if len(owed) == 0 {
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "nothing owed"))
		return
	}
	for item, n := range owed {
		if n > 0 {
			a.Inventory[item] += n
		}
	}
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "claimed"))
}
