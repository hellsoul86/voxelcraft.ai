package runtime

import (
	"sort"

	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
	"voxelcraft.ai/internal/sim/world/logic/ids"
)

func ContainerID(typ string, pos modelpkg.Vec3i) string {
	return ids.ContainerID(typ, pos.X, pos.Y, pos.Z)
}

func ParseContainerID(id string) (typ string, pos modelpkg.Vec3i, ok bool) {
	typ, x, y, z, ok := ids.ParseContainerID(id)
	if !ok {
		return "", modelpkg.Vec3i{}, false
	}
	return typ, modelpkg.Vec3i{X: x, Y: y, Z: z}, true
}

func EnsureContainer(containers map[modelpkg.Vec3i]*modelpkg.Container, pos modelpkg.Vec3i, typ string) *modelpkg.Container {
	c := containers[pos]
	if c != nil {
		c.Type = typ
		return c
	}
	c = &modelpkg.Container{
		Type:      typ,
		Pos:       pos,
		Inventory: map[string]int{},
	}
	containers[pos] = c
	return c
}

func RemoveContainer(containers map[modelpkg.Vec3i]*modelpkg.Container, pos modelpkg.Vec3i) *modelpkg.Container {
	c := containers[pos]
	if c == nil {
		return nil
	}
	delete(containers, pos)
	return c
}

func GetContainerByID(containers map[modelpkg.Vec3i]*modelpkg.Container, id string) *modelpkg.Container {
	typ, pos, ok := ParseContainerID(id)
	if !ok {
		return nil
	}
	c := containers[pos]
	if c == nil || c.Type != typ {
		return nil
	}
	return c
}

func SignIDAt(pos modelpkg.Vec3i) string {
	return ids.SignIDAt(pos.X, pos.Y, pos.Z)
}

func EnsureSign(signs map[modelpkg.Vec3i]*modelpkg.Sign, pos modelpkg.Vec3i) *modelpkg.Sign {
	s := signs[pos]
	if s != nil {
		s.Pos = pos
		return s
	}
	s = &modelpkg.Sign{Pos: pos}
	signs[pos] = s
	return s
}

func RemoveSign(signs map[modelpkg.Vec3i]*modelpkg.Sign, pos modelpkg.Vec3i) bool {
	if _, ok := signs[pos]; !ok {
		return false
	}
	delete(signs, pos)
	return true
}

func SortedSignPositionsNear(signs map[modelpkg.Vec3i]*modelpkg.Sign, pos modelpkg.Vec3i, dist int) []modelpkg.Vec3i {
	out := make([]modelpkg.Vec3i, 0, 8)
	for p, s := range signs {
		if s == nil {
			continue
		}
		if modelpkg.Manhattan(p, pos) <= dist {
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

func ConveyorIDAt(pos modelpkg.Vec3i) string {
	return ids.ConveyorIDAt(pos.X, pos.Y, pos.Z)
}

func EnsureConveyor(conveyors map[modelpkg.Vec3i]modelpkg.ConveyorMeta, pos modelpkg.Vec3i, dx, dz int) {
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
	if dx != 0 && dz != 0 {
		dz = 0
	}
	conveyors[pos] = modelpkg.ConveyorMeta{DX: int8(dx), DZ: int8(dz)}
}

func RemoveConveyor(conveyors map[modelpkg.Vec3i]modelpkg.ConveyorMeta, pos modelpkg.Vec3i) bool {
	if _, ok := conveyors[pos]; !ok {
		return false
	}
	delete(conveyors, pos)
	return true
}

func SortedConveyorPositionsNear(
	conveyors map[modelpkg.Vec3i]modelpkg.ConveyorMeta,
	pos modelpkg.Vec3i,
	dist int,
	blockNameAt func(modelpkg.Vec3i) string,
) []modelpkg.Vec3i {
	out := make([]modelpkg.Vec3i, 0, 8)
	for p := range conveyors {
		if modelpkg.Manhattan(p, pos) > dist {
			continue
		}
		if blockNameAt != nil && blockNameAt(p) != "CONVEYOR" {
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

func SwitchIDAt(pos modelpkg.Vec3i) string {
	return ids.SwitchIDAt(pos.X, pos.Y, pos.Z)
}

func EnsureSwitch(switches map[modelpkg.Vec3i]bool, pos modelpkg.Vec3i, on bool) {
	switches[pos] = on
}

func RemoveSwitch(switches map[modelpkg.Vec3i]bool, pos modelpkg.Vec3i) bool {
	if _, ok := switches[pos]; !ok {
		return false
	}
	delete(switches, pos)
	return true
}

func SortedSwitchPositionsNear(
	switches map[modelpkg.Vec3i]bool,
	pos modelpkg.Vec3i,
	dist int,
	blockNameAt func(modelpkg.Vec3i) string,
) []modelpkg.Vec3i {
	out := make([]modelpkg.Vec3i, 0, 8)
	for p := range switches {
		if modelpkg.Manhattan(p, pos) > dist {
			continue
		}
		if blockNameAt != nil && blockNameAt(p) != "SWITCH" {
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
