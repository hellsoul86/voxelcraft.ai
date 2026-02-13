package runtime

import (
	transferorgpkg "voxelcraft.ai/internal/sim/world/feature/transfer/org"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func SnapshotOrgMeta(orgs map[string]*modelpkg.Organization) OrgMetaResp {
	states := transferorgpkg.StatesFromOrganizations(orgs)
	return BuildOrgMetaResp(states)
}

type ApplyOrgMetaUpsertInput struct {
	Orgs     map[string]*modelpkg.Organization
	Agents   map[string]*modelpkg.Agent
	Incoming []transferorgpkg.State
	OnOrg    func(org *modelpkg.Organization)
}

func ApplyOrgMetaUpsert(in ApplyOrgMetaUpsertInput) {
	existingStates := transferorgpkg.StatesFromOrganizations(in.Orgs)
	mergedStates, ownerByAgent := BuildOrgMetaMerge(existingStates, in.Incoming)
	transferorgpkg.ApplyStates(in.Orgs, mergedStates, in.OnOrg)
	transferorgpkg.ReconcileAgentsOrg(in.Agents, in.Orgs, ownerByAgent)
}
