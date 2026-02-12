package instants

type Vec3 struct {
	X int
	Y int
	Z int
}

type TerminalContext struct {
	Type     string
	Distance int
	Matches  bool
}

func BuildTerminalContext(hasTerminal bool, terminalType string, terminalPos Vec3, expectedPos Vec3, agentPos Vec3) TerminalContext {
	if !hasTerminal {
		return TerminalContext{}
	}
	return TerminalContext{
		Type:     terminalType,
		Distance: manhattan(agentPos, terminalPos),
		Matches:  terminalPos == expectedPos,
	}
}

func manhattan(a, b Vec3) int {
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
