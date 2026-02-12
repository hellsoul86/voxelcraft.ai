package runtime

type Pos struct {
	X int
	Y int
	Z int
}

func MoveTolerance(tolerance float64) int {
	want := int(tolerance)
	if float64(want) < tolerance {
		want++
	}
	if want < 1 {
		want = 1
	}
	return want
}

func ShouldSkipStorm(weather string, nowTick uint64) bool {
	return weather == "STORM" && nowTick%2 == 1
}

func ShouldSkipFlood(activeEventID string, activeRadius int, nowTick uint64, activeEnds uint64, pos, center Pos) bool {
	if activeEventID != "FLOOD_WARNING" || activeRadius <= 0 || nowTick >= activeEnds {
		return false
	}
	return DistXZ(pos, center) <= activeRadius && nowTick%3 == 1
}

func DistXZ(a, b Pos) int {
	dx := a.X - b.X
	if dx < 0 {
		dx = -dx
	}
	dz := a.Z - b.Z
	if dz < 0 {
		dz = -dz
	}
	return dx + dz
}

func PrimaryAxis(dx, dz int) bool {
	return abs(dx) >= abs(dz)
}

func PrimaryStep(cur Pos, dx, dz int, primaryX bool) Pos {
	next := cur
	if primaryX {
		if dx > 0 {
			next.X++
		} else if dx < 0 {
			next.X--
		}
		return next
	}
	if dz > 0 {
		next.Z++
	} else if dz < 0 {
		next.Z--
	}
	return next
}

func SecondaryStep(cur Pos, dx, dz int, primaryX bool) Pos {
	next := cur
	if primaryX {
		if dz > 0 {
			next.Z++
		} else if dz < 0 {
			next.Z--
		}
		return next
	}
	if dx > 0 {
		next.X++
	} else if dx < 0 {
		next.X--
	}
	return next
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
