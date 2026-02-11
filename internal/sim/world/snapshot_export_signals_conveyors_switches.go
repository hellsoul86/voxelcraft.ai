package world

import (
	"sort"

	"voxelcraft.ai/internal/persistence/snapshot"
)

func (w *World) exportSnapshotSigns() []snapshot.SignV1 {
	signPos := make([]Vec3i, 0, len(w.signs))
	for p, s := range w.signs {
		if s == nil {
			continue
		}
		if w.blockName(w.chunks.GetBlock(p)) != "SIGN" {
			continue
		}
		signPos = append(signPos, p)
	}
	sort.Slice(signPos, func(i, j int) bool {
		if signPos[i].X != signPos[j].X {
			return signPos[i].X < signPos[j].X
		}
		if signPos[i].Y != signPos[j].Y {
			return signPos[i].Y < signPos[j].Y
		}
		return signPos[i].Z < signPos[j].Z
	})
	signSnaps := make([]snapshot.SignV1, 0, len(signPos))
	for _, p := range signPos {
		s := w.signs[p]
		if s == nil {
			continue
		}
		signSnaps = append(signSnaps, snapshot.SignV1{
			Pos:         p.ToArray(),
			Text:        s.Text,
			UpdatedTick: s.UpdatedTick,
			UpdatedBy:   s.UpdatedBy,
		})
	}
	return signSnaps
}

func (w *World) exportSnapshotConveyors() []snapshot.ConveyorV1 {
	conveyorPos := make([]Vec3i, 0, len(w.conveyors))
	for p := range w.conveyors {
		if w.blockName(w.chunks.GetBlock(p)) != "CONVEYOR" {
			continue
		}
		conveyorPos = append(conveyorPos, p)
	}
	sort.Slice(conveyorPos, func(i, j int) bool {
		if conveyorPos[i].X != conveyorPos[j].X {
			return conveyorPos[i].X < conveyorPos[j].X
		}
		if conveyorPos[i].Y != conveyorPos[j].Y {
			return conveyorPos[i].Y < conveyorPos[j].Y
		}
		return conveyorPos[i].Z < conveyorPos[j].Z
	})
	conveyorSnaps := make([]snapshot.ConveyorV1, 0, len(conveyorPos))
	for _, p := range conveyorPos {
		m := w.conveyors[p]
		conveyorSnaps = append(conveyorSnaps, snapshot.ConveyorV1{
			Pos: p.ToArray(),
			DX:  int(m.DX),
			DZ:  int(m.DZ),
		})
	}
	return conveyorSnaps
}

func (w *World) exportSnapshotSwitches() []snapshot.SwitchV1 {
	switchPos := make([]Vec3i, 0, len(w.switches))
	for p := range w.switches {
		if w.blockName(w.chunks.GetBlock(p)) != "SWITCH" {
			continue
		}
		switchPos = append(switchPos, p)
	}
	sort.Slice(switchPos, func(i, j int) bool {
		if switchPos[i].X != switchPos[j].X {
			return switchPos[i].X < switchPos[j].X
		}
		if switchPos[i].Y != switchPos[j].Y {
			return switchPos[i].Y < switchPos[j].Y
		}
		return switchPos[i].Z < switchPos[j].Z
	})
	switchSnaps := make([]snapshot.SwitchV1, 0, len(switchPos))
	for _, p := range switchPos {
		switchSnaps = append(switchSnaps, snapshot.SwitchV1{
			Pos: p.ToArray(),
			On:  w.switches[p],
		})
	}
	return switchSnaps
}
