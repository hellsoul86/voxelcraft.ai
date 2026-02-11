package movement

type Pos struct {
	X int
	Y int
	Z int
}

func distXZ(a, b Pos) int {
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

// DetourStep2D attempts to find a passable next step from start that eventually
// reduces the distance to target within maxDepth steps. It is intentionally small
// and deterministic (fixed neighbor order) to keep MOVE_TO stable.
//
// Returns (nextStep, true) on success. nextStep is always one of the 4-neighbors
// of start (same y=0 plane).
func DetourStep2D(start, target Pos, maxDepth int, inBounds func(Pos) bool, isSolid func(Pos) bool) (Pos, bool) {
	if maxDepth <= 0 {
		return Pos{}, false
	}
	start.Y = 0
	target.Y = 0

	startDist := distXZ(start, target)

	type qItem struct {
		p     Pos
		depth int
		first Pos
	}

	// Fixed neighbor order for determinism.
	dirs := []Pos{{X: 1}, {X: -1}, {Z: 1}, {Z: -1}}

	visited := make(map[Pos]bool, 256)
	visited[start] = true

	queue := make([]qItem, 0, 256)
	for _, d := range dirs {
		np := Pos{X: start.X + d.X, Y: 0, Z: start.Z + d.Z}
		if !inBounds(np) {
			continue
		}
		if isSolid(np) {
			continue
		}
		visited[np] = true
		queue = append(queue, qItem{p: np, depth: 1, first: np})
	}

	bestDist := startDist
	bestDepth := 0
	bestFirst := Pos{}
	found := false

	better := func(dist, depth int, first Pos) bool {
		if !found {
			return true
		}
		if dist != bestDist {
			return dist < bestDist
		}
		if depth != bestDepth {
			return depth < bestDepth
		}
		if first.X != bestFirst.X {
			return first.X < bestFirst.X
		}
		return first.Z < bestFirst.Z
	}

	for head := 0; head < len(queue); head++ {
		it := queue[head]

		d := distXZ(it.p, target)
		if d < startDist {
			if !found || better(d, it.depth, it.first) {
				found = true
				bestDist = d
				bestDepth = it.depth
				bestFirst = it.first
			}
		}

		if it.depth >= maxDepth {
			continue
		}
		for _, dir := range dirs {
			np := Pos{X: it.p.X + dir.X, Y: 0, Z: it.p.Z + dir.Z}
			if visited[np] {
				continue
			}
			if !inBounds(np) {
				continue
			}
			if isSolid(np) {
				continue
			}
			visited[np] = true
			queue = append(queue, qItem{p: np, depth: it.depth + 1, first: it.first})
		}
	}

	if !found {
		return Pos{}, false
	}
	return bestFirst, true
}
