package runtime

import orgpkg "voxelcraft.ai/internal/sim/world/feature/transfer/org"

type AgentPosReq struct {
	AgentID string
	Resp    chan AgentPosResp
}

type AgentPosResp struct {
	Pos [3]int
	Err string
}

type OrgMetaReq struct {
	Resp chan OrgMetaResp
}

type OrgMetaResp struct {
	Orgs []orgpkg.State
	Err  string
}

type OrgMetaUpsertReq struct {
	Orgs []orgpkg.State
	Resp chan OrgMetaUpsertResp
}

type OrgMetaUpsertResp struct {
	Err string
}
