package world

type sessionInstantsWorldEnv struct {
	w *World
}

func (e sessionInstantsWorldEnv) IsOrgMember(agentID, orgID string) bool {
	if e.w == nil {
		return false
	}
	return e.w.isOrgMember(agentID, orgID)
}

func (e sessionInstantsWorldEnv) PermissionsFor(agentID string, pos Vec3i) map[string]bool {
	if e.w == nil {
		return map[string]bool{}
	}
	_, perms := e.w.permissionsFor(agentID, pos)
	return perms
}

func (e sessionInstantsWorldEnv) BroadcastChat(nowTick uint64, from *Agent, channel, text string) {
	if e.w == nil {
		return
	}
	e.w.broadcastChat(nowTick, from, channel, text)
}

func (e sessionInstantsWorldEnv) AgentByID(agentID string) *Agent {
	if e.w == nil {
		return nil
	}
	return e.w.agents[agentID]
}
