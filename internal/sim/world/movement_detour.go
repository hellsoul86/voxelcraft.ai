package world

// detourStep2D attempts to find a passable next step from start that eventually
// reduces the distance to target within maxDepth steps. It is intentionally small
// and deterministic (fixed neighbor order) to keep MOVE_TO stable for agents.
//
// Returns (nextStep, true) on success. nextStep is always one of the 4-neighbors
// of start (same y=0 plane).
func (w *World) detourStep2D(start, target Vec3i, maxDepth int) (Vec3i, bool) {
	if maxDepth <= 0 {
		return Vec3i{}, false
	}
	start.Y = 0
	target.Y = 0

	startDist := distXZ(start, target)

	type qItem struct {
		p     Vec3i
		depth int
		first Vec3i
	}

	// Fixed neighbor order for determinism.
	dirs := []Vec3i{{X: 1}, {X: -1}, {Z: 1}, {Z: -1}}

	visited := make(map[Vec3i]bool, 256)
	visited[start] = true

	queue := make([]qItem, 0, 256)
	for _, d := range dirs {
		np := Vec3i{X: start.X + d.X, Y: 0, Z: start.Z + d.Z}
		if !w.chunks.inBounds(np) {
			continue
		}
		if w.blockSolid(w.chunks.GetBlock(np)) {
			continue
		}
		visited[np] = true
		queue = append(queue, qItem{p: np, depth: 1, first: np})
	}

	bestDist := startDist
	bestDepth := 0
	bestFirst := Vec3i{}
	found := false

	better := func(dist, depth int, first Vec3i) bool {
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
			np := Vec3i{X: it.p.X + dir.X, Y: 0, Z: it.p.Z + dir.Z}
			if visited[np] {
				continue
			}
			if !w.chunks.inBounds(np) {
				continue
			}
			if w.blockSolid(w.chunks.GetBlock(np)) {
				continue
			}
			visited[np] = true
			queue = append(queue, qItem{p: np, depth: it.depth + 1, first: it.first})
		}
	}

	if !found {
		return Vec3i{}, false
	}
	return bestFirst, true
}

