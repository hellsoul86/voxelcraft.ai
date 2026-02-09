package world

func clamp01k(x int) int {
	if x < 0 {
		return 0
	}
	if x > 1000 {
		return 1000
	}
	return x
}

func (w *World) repDepositMultiplier(a *Agent) int {
	if a == nil {
		return 1
	}
	// MVP heuristic: low trade reputation => higher required deposit.
	switch {
	case a.RepTrade >= 500:
		return 1
	case a.RepTrade >= 300:
		return 2
	default:
		return 3
	}
}

func (w *World) bumpRepTrade(agentID string, delta int) {
	a := w.agents[agentID]
	if a == nil {
		return
	}
	a.RepTrade = clamp01k(a.RepTrade + delta)
}

func (w *World) bumpRepBuild(agentID string, delta int) {
	a := w.agents[agentID]
	if a == nil {
		return
	}
	a.RepBuild = clamp01k(a.RepBuild + delta)
}

func (w *World) bumpRepSocial(agentID string, delta int) {
	a := w.agents[agentID]
	if a == nil {
		return
	}
	a.RepSocial = clamp01k(a.RepSocial + delta)
}

func (w *World) bumpRepLaw(agentID string, delta int) {
	a := w.agents[agentID]
	if a == nil {
		return
	}
	a.RepLaw = clamp01k(a.RepLaw + delta)
}
