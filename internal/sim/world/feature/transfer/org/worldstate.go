package org

import modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"

func StatesFromOrganizations(orgs map[string]*modelpkg.Organization) []State {
	if len(orgs) == 0 {
		return nil
	}
	states := make([]State, 0, len(orgs))
	for id, org := range orgs {
		if org == nil || id == "" {
			continue
		}
		members := map[string]string{}
		for aid, role := range org.Members {
			members[aid] = string(role)
		}
		states = append(states, State{
			OrgID:       org.OrgID,
			Kind:        string(org.Kind),
			Name:        org.Name,
			CreatedTick: org.CreatedTick,
			MetaVersion: org.MetaVersion,
			Members:     members,
		})
	}
	return states
}

func ApplyStates(
	orgs map[string]*modelpkg.Organization,
	states []State,
	ensureTreasury func(*modelpkg.Organization),
) {
	for _, src := range states {
		dst := orgs[src.OrgID]
		if dst == nil {
			dst = &modelpkg.Organization{
				OrgID:           src.OrgID,
				Treasury:        map[string]int{},
				TreasuryByWorld: map[string]map[string]int{},
			}
			orgs[src.OrgID] = dst
		}
		dst.Kind = modelpkg.OrgKind(src.Kind)
		dst.Name = src.Name
		dst.CreatedTick = src.CreatedTick
		dst.MetaVersion = src.MetaVersion
		nextMembers := make(map[string]modelpkg.OrgRole, len(src.Members))
		for aid, role := range src.Members {
			nextMembers[aid] = modelpkg.OrgRole(role)
		}
		dst.Members = nextMembers
		if ensureTreasury != nil {
			ensureTreasury(dst)
		}
	}
}

func ReconcileAgentsOrg(
	agents map[string]*modelpkg.Agent,
	orgs map[string]*modelpkg.Organization,
	ownerByAgent map[string]string,
) {
	for _, a := range agents {
		if a == nil {
			continue
		}
		if orgID, ok := ownerByAgent[a.ID]; ok {
			a.OrgID = orgID
			continue
		}
		if a.OrgID == "" {
			continue
		}
		org := orgs[a.OrgID]
		if org == nil || org.Members == nil || org.Members[a.ID] == "" {
			a.OrgID = ""
		}
	}
}
