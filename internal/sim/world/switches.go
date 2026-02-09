package world

import "sort"

func switchIDAt(pos Vec3i) string { return containerID("SWITCH", pos) }

func (w *World) ensureSwitch(pos Vec3i, on bool) {
	if w.switches == nil {
		w.switches = map[Vec3i]bool{}
	}
	w.switches[pos] = on
}

func (w *World) removeSwitch(nowTick uint64, actor string, pos Vec3i, reason string) {
	if w.switches == nil {
		return
	}
	if _, ok := w.switches[pos]; !ok {
		return
	}
	delete(w.switches, pos)
	w.auditEvent(nowTick, actor, "SWITCH_REMOVE", pos, reason, map[string]any{
		"switch_id": switchIDAt(pos),
	})
}

func (w *World) sortedSwitchPositionsNear(pos Vec3i, dist int) []Vec3i {
	if len(w.switches) == 0 {
		return nil
	}
	out := make([]Vec3i, 0, 8)
	for p := range w.switches {
		if Manhattan(p, pos) > dist {
			continue
		}
		// Guard against stale meta.
		if w.blockName(w.chunks.GetBlock(p)) != "SWITCH" {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].X != out[j].X {
			return out[i].X < out[j].X
		}
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].Z < out[j].Z
	})
	return out
}
