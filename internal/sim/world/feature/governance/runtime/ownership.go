package runtime

import (
	"sort"

	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func LandAt(claims map[string]*modelpkg.LandClaim, pos modelpkg.Vec3i) *modelpkg.LandClaim {
	for _, c := range claims {
		if c != nil && c.Contains(pos) {
			return c
		}
	}
	return nil
}

func IsOrgMember(orgs map[string]*modelpkg.Organization, agentID, orgID string) bool {
	if orgID == "" {
		return false
	}
	o := orgs[orgID]
	if o == nil || o.Members == nil {
		return false
	}
	_, ok := o.Members[agentID]
	return ok
}

func IsOrgAdmin(orgs map[string]*modelpkg.Organization, agentID, orgID string) bool {
	if orgID == "" {
		return false
	}
	o := orgs[orgID]
	if o == nil || o.Members == nil {
		return false
	}
	role := o.Members[agentID]
	return role == modelpkg.OrgLeader || role == modelpkg.OrgOfficer
}

func IsLandAdmin(orgs map[string]*modelpkg.Organization, agentID string, land *modelpkg.LandClaim) bool {
	if land == nil {
		return false
	}
	if land.Owner == agentID {
		return true
	}
	return IsOrgAdmin(orgs, agentID, land.Owner)
}

func IsLandMember(orgs map[string]*modelpkg.Organization, agentID string, land *modelpkg.LandClaim) bool {
	if land == nil {
		return false
	}
	if land.Owner == agentID {
		return true
	}
	if land.Members != nil && land.Members[agentID] {
		return true
	}
	return IsOrgMember(orgs, agentID, land.Owner)
}

func SortedClaimIDs(claims map[string]*modelpkg.LandClaim) []string {
	ids := make([]string, 0, len(claims))
	for id := range claims {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
