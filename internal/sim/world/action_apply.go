package world

import "voxelcraft.ai/internal/protocol"

func (w *World) applyAct(a *Agent, act protocol.ActMsg, nowTick uint64) {
	// Staleness check: accept only [now-2, now].
	if act.Tick+2 < nowTick || act.Tick > nowTick {
		a.AddEvent(actionResult(nowTick, "ACT", false, "E_STALE", "act tick out of range"))
		return
	}

	// Cancel first.
	for _, cid := range act.Cancel {
		if a.MoveTask != nil && a.MoveTask.TaskID == cid {
			a.MoveTask = nil
			a.AddEvent(actionResult(nowTick, cid, true, "", "canceled"))
			continue
		}
		if a.WorkTask != nil && a.WorkTask.TaskID == cid {
			a.WorkTask = nil
			a.AddEvent(actionResult(nowTick, cid, true, "", "canceled"))
			continue
		}
		a.AddEvent(actionResult(nowTick, cid, false, "E_INVALID_TARGET", "task not found"))
	}

	// Instants.
	for _, inst := range act.Instants {
		w.applyInstant(a, inst, nowTick)
	}

	// Tasks.
	for _, tr := range act.Tasks {
		w.applyTaskReq(a, tr, nowTick)
	}
}

func (w *World) applyInstant(a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if h := instantDispatch[inst.Type]; h != nil {
		h(w, a, inst, nowTick)
		return
	}
	a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "unknown instant type"))
}

func (w *World) applyTaskReq(a *Agent, tr protocol.TaskReq, nowTick uint64) {
	if h := taskReqDispatch[tr.Type]; h != nil {
		h(w, a, tr, nowTick)
		return
	}
	a.AddEvent(actionResult(nowTick, tr.ID, false, "E_BAD_REQUEST", "unknown task type"))
}

func actionResult(tick uint64, ref string, ok bool, code string, message string) protocol.Event {
	e := protocol.Event{
		"t":    tick,
		"type": "ACTION_RESULT",
		"ref":  ref,
		"ok":   ok,
	}
	if code != "" {
		e["code"] = code
	}
	if message != "" {
		e["message"] = message
	}
	return e
}
