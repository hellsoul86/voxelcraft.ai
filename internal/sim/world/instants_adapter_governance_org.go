package world

type governanceOrgInstantsWorldEnv struct {
	w *World
}

func (e governanceOrgInstantsWorldEnv) NewOrgID() string {
	if e.w == nil {
		return ""
	}
	return e.w.newOrgID()
}

func (e governanceOrgInstantsWorldEnv) GetOrg(orgID string) *Organization {
	if e.w == nil {
		return nil
	}
	return e.w.orgByID(orgID)
}

func (e governanceOrgInstantsWorldEnv) PutOrg(org *Organization) {
	if e.w == nil || org == nil {
		return
	}
	e.w.orgs[org.OrgID] = org
}

func (e governanceOrgInstantsWorldEnv) DeleteOrg(orgID string) {
	if e.w == nil {
		return
	}
	delete(e.w.orgs, orgID)
}

func (e governanceOrgInstantsWorldEnv) OrgTreasury(org *Organization) map[string]int {
	if e.w == nil {
		return nil
	}
	return e.w.orgTreasury(org)
}

func (e governanceOrgInstantsWorldEnv) IsOrgMember(agentID string, orgID string) bool {
	if e.w == nil {
		return false
	}
	return e.w.isOrgMember(agentID, orgID)
}

func (e governanceOrgInstantsWorldEnv) IsOrgAdmin(agentID string, orgID string) bool {
	if e.w == nil {
		return false
	}
	return e.w.isOrgAdmin(agentID, orgID)
}

func (e governanceOrgInstantsWorldEnv) AuditOrgEvent(nowTick uint64, actorID string, action string, reason string, details map[string]any) {
	if e.w == nil {
		return
	}
	pos := Vec3i{}
	if a := e.w.agents[actorID]; a != nil {
		pos = a.Pos
	}
	e.w.auditEvent(nowTick, actorID, action, pos, reason, details)
}
