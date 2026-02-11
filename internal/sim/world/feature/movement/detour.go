package movement

import logicmovement "voxelcraft.ai/internal/sim/world/logic/movement"

type Pos struct {
	X int
	Y int
	Z int
}

func DetourStep2D(start, target Pos, maxDepth int, inBounds func(Pos) bool, isSolid func(Pos) bool) (Pos, bool) {
	next, ok := logicmovement.DetourStep2D(
		logicmovement.Pos{X: start.X, Y: start.Y, Z: start.Z},
		logicmovement.Pos{X: target.X, Y: target.Y, Z: target.Z},
		maxDepth,
		func(p logicmovement.Pos) bool {
			if inBounds == nil {
				return true
			}
			return inBounds(Pos{X: p.X, Y: p.Y, Z: p.Z})
		},
		func(p logicmovement.Pos) bool {
			if isSolid == nil {
				return false
			}
			return isSolid(Pos{X: p.X, Y: p.Y, Z: p.Z})
		},
	)
	if !ok {
		return Pos{}, false
	}
	return Pos{X: next.X, Y: next.Y, Z: next.Z}, true
}
