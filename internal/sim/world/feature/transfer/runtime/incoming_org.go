package runtime

import modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"

// UpsertIncomingOrg applies incoming org transfer metadata into the world org map and
// ensures the agent is a member. It returns the effective org, or nil when no org is needed.
func UpsertIncomingOrg(
	orgs map[string]*modelpkg.Organization,
	incoming *OrgTransfer,
	fallbackOrgID string,
	agentID string,
) *modelpkg.Organization {
	if orgs == nil {
		return nil
	}
	orgID := fallbackOrgID
	if incoming != nil && incoming.OrgID != "" {
		orgID = incoming.OrgID
	}
	if orgID == "" {
		return nil
	}

	org := orgs[orgID]
	if org == nil {
		org = &modelpkg.Organization{
			OrgID:           orgID,
			Kind:            modelpkg.OrgGuild,
			Name:            orgID,
			Members:         map[string]modelpkg.OrgRole{},
			Treasury:        map[string]int{},
			TreasuryByWorld: map[string]map[string]int{},
		}
		orgs[orgID] = org
	}

	if incoming != nil {
		if incoming.Kind != "" {
			org.Kind = incoming.Kind
		}
		if incoming.Name != "" {
			org.Name = incoming.Name
		}
		if org.CreatedTick == 0 {
			org.CreatedTick = incoming.CreatedTick
		}
		if incoming.MetaVersion > org.MetaVersion {
			org.MetaVersion = incoming.MetaVersion
		}
		if org.Members == nil {
			org.Members = map[string]modelpkg.OrgRole{}
		}
		for aid, role := range incoming.Members {
			if aid == "" || role == "" {
				continue
			}
			org.Members[aid] = role
		}
	}

	if org.Members == nil {
		org.Members = map[string]modelpkg.OrgRole{}
	}
	if agentID != "" {
		if _, ok := org.Members[agentID]; !ok {
			org.Members[agentID] = modelpkg.OrgMember
		}
	}
	return org
}
