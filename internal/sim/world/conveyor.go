package world

import (
	"sort"

	"voxelcraft.ai/internal/sim/world/logic/ids"
)

// ConveyorMeta stores minimal runtime metadata for a conveyor block.
// We keep it intentionally small and deterministic: a single cardinal direction.
type ConveyorMeta struct {
	DX int8 // -1,0,1
	DZ int8 // -1,0,1
}

func conveyorIDAt(pos Vec3i) string { return ids.ConveyorIDAt(pos.X, pos.Y, pos.Z) }

func (w *World) ensureConveyor(pos Vec3i, dx, dz int) {
	if dx > 1 {
		dx = 1
	} else if dx < -1 {
		dx = -1
	}
	if dz > 1 {
		dz = 1
	} else if dz < -1 {
		dz = -1
	}
	// Enforce cardinal direction (deterministic tie-break).
	if dx != 0 && dz != 0 {
		dz = 0
	}
	w.conveyors[pos] = ConveyorMeta{DX: int8(dx), DZ: int8(dz)}
}

func (w *World) removeConveyor(nowTick uint64, actor string, pos Vec3i, reason string) {
	if _, ok := w.conveyors[pos]; !ok {
		return
	}
	delete(w.conveyors, pos)
	w.auditEvent(nowTick, actor, "CONVEYOR_REMOVE", pos, reason, map[string]any{
		"conveyor_id": conveyorIDAt(pos),
	})
}

func (w *World) sortedConveyorPositionsNear(pos Vec3i, dist int) []Vec3i {
	out := make([]Vec3i, 0, 8)
	for p := range w.conveyors {
		if Manhattan(p, pos) > dist {
			continue
		}
		// Guard against stale meta (e.g. if blocks were edited without calling removeConveyor).
		if w.blockName(w.chunks.GetBlock(p)) != "CONVEYOR" {
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

func conveyorDirTag(m ConveyorMeta) string {
	switch {
	case m.DX == 1 && m.DZ == 0:
		return "+X"
	case m.DX == -1 && m.DZ == 0:
		return "-X"
	case m.DX == 0 && m.DZ == 1:
		return "+Z"
	case m.DX == 0 && m.DZ == -1:
		return "-Z"
	default:
		return "?"
	}
}

func yawToDir(yaw int) (dx, dz int) {
	y := yaw % 360
	if y < 0 {
		y += 360
	}
	// Nearest 90 degrees.
	dir := ((y + 45) / 90) % 4
	switch dir {
	case 0:
		return 0, 1 // +Z
	case 1:
		return 1, 0 // +X
	case 2:
		return 0, -1 // -Z
	default:
		return -1, 0 // -X
	}
}
