package world

import lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"

type governanceLawInstantsWorldEnv struct {
	w *World
}

func (e governanceLawInstantsWorldEnv) GetLand(landID string) *LandClaim {
	if e.w == nil {
		return nil
	}
	return e.w.claims[landID]
}

func (e governanceLawInstantsWorldEnv) IsLandMember(agentID string, land *LandClaim) bool {
	if e.w == nil {
		return false
	}
	return e.w.isLandMember(agentID, land)
}

func (e governanceLawInstantsWorldEnv) GetLawTemplateTitle(templateID string) (string, bool) {
	if e.w == nil {
		return "", false
	}
	tmpl, ok := e.w.catalogs.Laws.ByID[templateID]
	if !ok {
		return "", false
	}
	return tmpl.Title, true
}

func (e governanceLawInstantsWorldEnv) ItemExists(itemID string) bool {
	if e.w == nil {
		return false
	}
	_, ok := e.w.catalogs.Items.Defs[itemID]
	return ok
}

func (e governanceLawInstantsWorldEnv) NewLawID() string {
	if e.w == nil {
		return ""
	}
	return e.w.newLawID()
}

func (e governanceLawInstantsWorldEnv) PutLaw(law *lawspkg.Law) {
	if e.w == nil || law == nil {
		return
	}
	e.w.laws[law.LawID] = law
}

func (e governanceLawInstantsWorldEnv) GetLaw(lawID string) *lawspkg.Law {
	if e.w == nil {
		return nil
	}
	return e.w.laws[lawID]
}

func (e governanceLawInstantsWorldEnv) BroadcastLawEvent(nowTick uint64, stage string, law *lawspkg.Law, note string) {
	if e.w == nil {
		return
	}
	e.w.broadcastLawEvent(nowTick, stage, law, note)
}

func (e governanceLawInstantsWorldEnv) AuditLawEvent(nowTick uint64, actorID string, action string, pos Vec3i, reason string, details map[string]any) {
	if e.w == nil {
		return
	}
	e.w.auditEvent(nowTick, actorID, action, pos, reason, details)
}
