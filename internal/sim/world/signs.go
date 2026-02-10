package world

import (
	"sort"
)

type Sign struct {
	Pos         Vec3i
	Text        string
	UpdatedTick uint64
	UpdatedBy   string
}

func signIDAt(pos Vec3i) string {
	return containerID("SIGN", pos)
}

func (w *World) ensureSign(pos Vec3i) *Sign {
	s := w.signs[pos]
	if s != nil {
		s.Pos = pos
		return s
	}
	s = &Sign{Pos: pos}
	w.signs[pos] = s
	return s
}

func (w *World) removeSign(nowTick uint64, actor string, pos Vec3i, reason string) {
	s := w.signs[pos]
	if s == nil {
		return
	}
	delete(w.signs, pos)
	// Record the removal as a separate audit event (the SET_BLOCK audit already exists too).
	w.auditEvent(nowTick, actor, "SIGN_REMOVE", pos, reason, map[string]any{
		"sign_id": signIDAt(pos),
	})
}

func (w *World) sortedSignPositionsNear(pos Vec3i, dist int) []Vec3i {
	out := make([]Vec3i, 0, 8)
	for p, s := range w.signs {
		if s == nil {
			continue
		}
		if Manhattan(p, pos) <= dist {
			out = append(out, p)
		}
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
