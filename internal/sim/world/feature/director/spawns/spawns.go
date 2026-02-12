package spawns

import "sort"

type Pos struct {
	X int
	Y int
	Z int
}

func Square(center Pos, radius int) []Pos {
	if radius < 0 {
		radius = 0
	}
	out := make([]Pos, 0, (2*radius+1)*(2*radius+1))
	for dz := -radius; dz <= radius; dz++ {
		for dx := -radius; dx <= radius; dx++ {
			out = append(out, Pos{X: center.X + dx, Y: center.Y, Z: center.Z + dz})
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

func Diamond(center Pos, manhattan int) []Pos {
	if manhattan < 0 {
		manhattan = 0
	}
	out := make([]Pos, 0, (2*manhattan+1)*(2*manhattan+1))
	for dz := -manhattan; dz <= manhattan; dz++ {
		for dx := -manhattan; dx <= manhattan; dx++ {
			abs := dx
			if abs < 0 {
				abs = -abs
			}
			abs2 := dz
			if abs2 < 0 {
				abs2 = -abs2
			}
			if abs+abs2 > manhattan {
				continue
			}
			out = append(out, Pos{X: center.X + dx, Y: center.Y, Z: center.Z + dz})
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

// RingSquare returns all cells on the square ring at exact radius.
func RingSquare(center Pos, radius int) []Pos {
	if radius <= 0 {
		return []Pos{center}
	}
	out := make([]Pos, 0, radius*8)
	for dz := -radius; dz <= radius; dz++ {
		for dx := -radius; dx <= radius; dx++ {
			adx := dx
			if adx < 0 {
				adx = -adx
			}
			adz := dz
			if adz < 0 {
				adz = -adz
			}
			if adx != radius && adz != radius {
				continue
			}
			out = append(out, Pos{X: center.X + dx, Y: center.Y, Z: center.Z + dz})
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

func DeepVeinIsCopper(x, z int) bool {
	return (x+z)&1 == 0
}
