package world

type Vec3i struct {
	X int
	Y int
	Z int
}

func (v Vec3i) ToArray() [3]int { return [3]int{v.X, v.Y, v.Z} }

func Manhattan(a, b Vec3i) int {
	dx := a.X - b.X
	if dx < 0 {
		dx = -dx
	}
	dy := a.Y - b.Y
	if dy < 0 {
		dy = -dy
	}
	dz := a.Z - b.Z
	if dz < 0 {
		dz = -dz
	}
	return dx + dy + dz
}
