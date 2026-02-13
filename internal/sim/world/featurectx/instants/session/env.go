package session

import modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"

type Env struct {
	IsOrgMemberFn    func(agentID, orgID string) bool
	PermissionsForFn func(agentID string, pos modelpkg.Vec3i) map[string]bool
	BroadcastChatFn  func(nowTick uint64, from *modelpkg.Agent, channel, text string)
	AgentByIDFn      func(agentID string) *modelpkg.Agent
}

func (e Env) IsOrgMember(agentID, orgID string) bool {
	if e.IsOrgMemberFn == nil {
		return false
	}
	return e.IsOrgMemberFn(agentID, orgID)
}

func (e Env) PermissionsFor(agentID string, pos modelpkg.Vec3i) map[string]bool {
	if e.PermissionsForFn == nil {
		return map[string]bool{}
	}
	return e.PermissionsForFn(agentID, pos)
}

func (e Env) BroadcastChat(nowTick uint64, from *modelpkg.Agent, channel, text string) {
	if e.BroadcastChatFn != nil {
		e.BroadcastChatFn(nowTick, from, channel, text)
	}
}

func (e Env) AgentByID(agentID string) *modelpkg.Agent {
	if e.AgentByIDFn == nil {
		return nil
	}
	return e.AgentByIDFn(agentID)
}
