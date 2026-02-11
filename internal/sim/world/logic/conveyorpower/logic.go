package conveyorpower

type Pos struct {
	X int
	Y int
	Z int
}

type Env interface {
	BlockName(Pos) string
	SwitchOn(Pos) bool
	SensorOn(Pos) bool
}

var cardinalDirs = []Pos{
	{X: 1, Y: 0, Z: 0},
	{X: -1, Y: 0, Z: 0},
	{X: 0, Y: 0, Z: 1},
	{X: 0, Y: 0, Z: -1},
}

func Enabled(env Env, pos Pos, maxNodes int) bool {
	// Rule 1: adjacent control blocks act as direct enable signals.
	// If any adjacent switch/sensor exists, require at least one to be ON.
	foundControl := false
	for _, d := range cardinalDirs {
		p := Pos{X: pos.X + d.X, Y: pos.Y + d.Y, Z: pos.Z + d.Z}
		switch env.BlockName(p) {
		case "SWITCH":
			foundControl = true
			if env.SwitchOn(p) {
				return true
			}
		case "SENSOR":
			foundControl = true
			if env.SensorOn(p) {
				return true
			}
		}
	}
	if foundControl {
		return false
	}

	// Rule 2: adjacent wires form a simple network; if any adjacent wire exists, require the
	// wire network to connect to an ON switch or active sensor (within a capped BFS budget).
	wireStarts := make([]Pos, 0, 4)
	for _, d := range cardinalDirs {
		p := Pos{X: pos.X + d.X, Y: pos.Y + d.Y, Z: pos.Z + d.Z}
		if env.BlockName(p) == "WIRE" {
			wireStarts = append(wireStarts, p)
		}
	}
	if len(wireStarts) > 0 {
		return wirePoweredBySwitch(env, wireStarts, maxNodes)
	}

	// No control blocks nearby -> enabled by default.
	return true
}

func wirePoweredBySwitch(env Env, starts []Pos, maxNodes int) bool {
	if len(starts) == 0 || maxNodes <= 0 {
		return false
	}

	visited := map[Pos]bool{}
	q := make([]Pos, 0, len(starts))
	for _, p := range starts {
		if env.BlockName(p) != "WIRE" {
			continue
		}
		if visited[p] {
			continue
		}
		visited[p] = true
		q = append(q, p)
	}

	for len(q) > 0 && len(visited) <= maxNodes {
		p := q[0]
		q = q[1:]

		// Check adjacent switches/sensors.
		for _, d := range cardinalDirs {
			sp := Pos{X: p.X + d.X, Y: p.Y + d.Y, Z: p.Z + d.Z}
			switch env.BlockName(sp) {
			case "SWITCH":
				if env.SwitchOn(sp) {
					return true
				}
			case "SENSOR":
				if env.SensorOn(sp) {
					return true
				}
			}
		}

		// Expand to neighboring wires.
		for _, d := range cardinalDirs {
			np := Pos{X: p.X + d.X, Y: p.Y + d.Y, Z: p.Z + d.Z}
			if visited[np] {
				continue
			}
			if env.BlockName(np) != "WIRE" {
				continue
			}
			visited[np] = true
			q = append(q, np)
			if len(visited) > maxNodes {
				break
			}
		}
	}
	return false
}
