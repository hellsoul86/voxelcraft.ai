package runtime

import (
	eventspkg "voxelcraft.ai/internal/sim/world/feature/transfer/events"
	orgpkg "voxelcraft.ai/internal/sim/world/feature/transfer/org"
)

func HandleEventsReq(req eventspkg.Req, load func(agentID string, sinceCursor uint64, limit int) ([]eventspkg.CursorItem, uint64, bool)) eventspkg.Resp {
	if load == nil {
		return eventspkg.Resp{Err: "events loader unavailable", NextCursor: req.SinceCursor}
	}
	items, next, ok := load(req.AgentID, req.SinceCursor, req.Limit)
	return eventspkg.BuildResp(ok, req.SinceCursor, items, next)
}

func HandleAgentPosReq(req AgentPosReq, lookup func(agentID string) ([3]int, bool)) AgentPosResp {
	if lookup == nil {
		return AgentPosResp{Err: "agent position lookup unavailable"}
	}
	pos, ok := lookup(req.AgentID)
	if !ok {
		return AgentPosResp{Err: "agent not found"}
	}
	return AgentPosResp{Pos: pos}
}

func BuildOrgMetaResp(states []orgpkg.State) OrgMetaResp {
	return OrgMetaResp{Orgs: orgpkg.NormalizeStates(states)}
}

func BuildOrgMetaMerge(existing []orgpkg.State, incoming []orgpkg.State) ([]orgpkg.State, map[string]string) {
	return orgpkg.MergeStates(existing, incoming)
}
