package world

import "fmt"

type OrgKind string

const (
	OrgGuild OrgKind = "GUILD"
	OrgCity  OrgKind = "CITY"
)

type OrgRole string

const (
	OrgLeader  OrgRole = "LEADER"
	OrgOfficer OrgRole = "OFFICER"
	OrgMember  OrgRole = "MEMBER"
)

type Organization struct {
	OrgID       string
	Kind        OrgKind
	Name        string
	CreatedTick uint64
	MetaVersion uint64

	Members         map[string]OrgRole // agent_id -> role
	Treasury        map[string]int
	TreasuryByWorld map[string]map[string]int
}

func (o *Organization) treasuryFor(worldID string) map[string]int {
	if o == nil {
		return nil
	}
	if worldID == "" {
		worldID = "GLOBAL"
	}
	if o.TreasuryByWorld == nil {
		o.TreasuryByWorld = map[string]map[string]int{}
	}
	m := o.TreasuryByWorld[worldID]
	if m == nil {
		m = map[string]int{}
		// One-time migration from legacy single-world treasury.
		// Only seed the very first world map; additional worlds start with empty treasuries.
		if len(o.TreasuryByWorld) == 0 {
			for item, n := range o.Treasury {
				if item == "" || n <= 0 {
					continue
				}
				m[item] = n
			}
		}
		o.TreasuryByWorld[worldID] = m
	}
	// Keep legacy field as a view for the currently accessed world to preserve existing callers/tests.
	o.Treasury = m
	return m
}

func (w *World) orgTreasury(o *Organization) map[string]int {
	if o == nil {
		return nil
	}
	return o.treasuryFor(w.cfg.ID)
}

func (w *World) newOrgID() string {
	n := w.nextOrgNum.Add(1)
	return fmt.Sprintf("ORG%06d", n)
}

func (w *World) orgByID(id string) *Organization {
	if id == "" {
		return nil
	}
	return w.orgs[id]
}

func (w *World) isOrgMember(agentID, orgID string) bool {
	o := w.orgByID(orgID)
	if o == nil || o.Members == nil {
		return false
	}
	_, ok := o.Members[agentID]
	return ok
}

func (w *World) isOrgAdmin(agentID, orgID string) bool {
	o := w.orgByID(orgID)
	if o == nil || o.Members == nil {
		return false
	}
	role := o.Members[agentID]
	return role == OrgLeader || role == OrgOfficer
}

func (w *World) isLandAdmin(agentID string, land *LandClaim) bool {
	if land == nil {
		return false
	}
	if land.Owner == agentID {
		return true
	}
	// If an org owns the land, leaders/officers are admins.
	return w.isOrgAdmin(agentID, land.Owner)
}

func (w *World) isLandMember(agentID string, land *LandClaim) bool {
	if land == nil {
		return false
	}
	if land.Owner == agentID {
		return true
	}
	if land.Members != nil && land.Members[agentID] {
		return true
	}
	// If an org owns the land, any org member is treated as land member.
	return w.isOrgMember(agentID, land.Owner)
}
