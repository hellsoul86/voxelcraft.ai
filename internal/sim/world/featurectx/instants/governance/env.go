package governance

import (
	governanceinstantspkg "voxelcraft.ai/internal/sim/world/feature/governance/instants"
	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type ClaimEnv struct {
	GetLandFn         func(landID string) *modelpkg.LandClaim
	IsLandAdminFn     func(agentID string, land *modelpkg.LandClaim) bool
	BlockNameAtFn     func(pos modelpkg.Vec3i) string
	ClaimRecordsFn    func() []governanceinstantspkg.ClaimRecord
	OwnerExistsFn     func(ownerID string) bool
	AuditClaimEventFn func(nowTick uint64, actorID string, action string, pos modelpkg.Vec3i, reason string, details map[string]any)
}

func (e ClaimEnv) GetLand(landID string) *modelpkg.LandClaim {
	if e.GetLandFn == nil {
		return nil
	}
	return e.GetLandFn(landID)
}

func (e ClaimEnv) IsLandAdmin(agentID string, land *modelpkg.LandClaim) bool {
	if e.IsLandAdminFn == nil {
		return false
	}
	return e.IsLandAdminFn(agentID, land)
}

func (e ClaimEnv) BlockNameAt(pos modelpkg.Vec3i) string {
	if e.BlockNameAtFn == nil {
		return ""
	}
	return e.BlockNameAtFn(pos)
}

func (e ClaimEnv) ClaimRecords() []governanceinstantspkg.ClaimRecord {
	if e.ClaimRecordsFn == nil {
		return nil
	}
	return e.ClaimRecordsFn()
}

func (e ClaimEnv) OwnerExists(ownerID string) bool {
	if e.OwnerExistsFn == nil {
		return false
	}
	return e.OwnerExistsFn(ownerID)
}

func (e ClaimEnv) AuditClaimEvent(nowTick uint64, actorID string, action string, pos modelpkg.Vec3i, reason string, details map[string]any) {
	if e.AuditClaimEventFn != nil {
		e.AuditClaimEventFn(nowTick, actorID, action, pos, reason, details)
	}
}

type LawEnv struct {
	GetLandFn             func(landID string) *modelpkg.LandClaim
	IsLandMemberFn        func(agentID string, land *modelpkg.LandClaim) bool
	GetLawTemplateTitleFn func(templateID string) (string, bool)
	ItemExistsFn          func(itemID string) bool
	NewLawIDFn            func() string
	PutLawFn              func(law *lawspkg.Law)
	GetLawFn              func(lawID string) *lawspkg.Law
	BroadcastLawEventFn   func(nowTick uint64, stage string, law *lawspkg.Law, note string)
	AuditLawEventFn       func(nowTick uint64, actorID string, action string, pos modelpkg.Vec3i, reason string, details map[string]any)
}

func (e LawEnv) GetLand(landID string) *modelpkg.LandClaim {
	if e.GetLandFn == nil {
		return nil
	}
	return e.GetLandFn(landID)
}

func (e LawEnv) IsLandMember(agentID string, land *modelpkg.LandClaim) bool {
	if e.IsLandMemberFn == nil {
		return false
	}
	return e.IsLandMemberFn(agentID, land)
}

func (e LawEnv) GetLawTemplateTitle(templateID string) (string, bool) {
	if e.GetLawTemplateTitleFn == nil {
		return "", false
	}
	return e.GetLawTemplateTitleFn(templateID)
}

func (e LawEnv) ItemExists(itemID string) bool {
	if e.ItemExistsFn == nil {
		return false
	}
	return e.ItemExistsFn(itemID)
}

func (e LawEnv) NewLawID() string {
	if e.NewLawIDFn == nil {
		return ""
	}
	return e.NewLawIDFn()
}

func (e LawEnv) PutLaw(law *lawspkg.Law) {
	if e.PutLawFn != nil {
		e.PutLawFn(law)
	}
}

func (e LawEnv) GetLaw(lawID string) *lawspkg.Law {
	if e.GetLawFn == nil {
		return nil
	}
	return e.GetLawFn(lawID)
}

func (e LawEnv) BroadcastLawEvent(nowTick uint64, stage string, law *lawspkg.Law, note string) {
	if e.BroadcastLawEventFn != nil {
		e.BroadcastLawEventFn(nowTick, stage, law, note)
	}
}

func (e LawEnv) AuditLawEvent(nowTick uint64, actorID string, action string, pos modelpkg.Vec3i, reason string, details map[string]any) {
	if e.AuditLawEventFn != nil {
		e.AuditLawEventFn(nowTick, actorID, action, pos, reason, details)
	}
}

type OrgEnv struct {
	NewOrgIDFn      func() string
	GetOrgFn        func(orgID string) *modelpkg.Organization
	PutOrgFn        func(org *modelpkg.Organization)
	DeleteOrgFn     func(orgID string)
	OrgTreasuryFn   func(org *modelpkg.Organization) map[string]int
	IsOrgMemberFn   func(agentID string, orgID string) bool
	IsOrgAdminFn    func(agentID string, orgID string) bool
	AuditOrgEventFn func(nowTick uint64, actorID string, action string, reason string, details map[string]any)
}

func (e OrgEnv) NewOrgID() string {
	if e.NewOrgIDFn == nil {
		return ""
	}
	return e.NewOrgIDFn()
}

func (e OrgEnv) GetOrg(orgID string) *modelpkg.Organization {
	if e.GetOrgFn == nil {
		return nil
	}
	return e.GetOrgFn(orgID)
}

func (e OrgEnv) PutOrg(org *modelpkg.Organization) {
	if e.PutOrgFn != nil {
		e.PutOrgFn(org)
	}
}

func (e OrgEnv) DeleteOrg(orgID string) {
	if e.DeleteOrgFn != nil {
		e.DeleteOrgFn(orgID)
	}
}

func (e OrgEnv) OrgTreasury(org *modelpkg.Organization) map[string]int {
	if e.OrgTreasuryFn == nil {
		return nil
	}
	return e.OrgTreasuryFn(org)
}

func (e OrgEnv) IsOrgMember(agentID string, orgID string) bool {
	if e.IsOrgMemberFn == nil {
		return false
	}
	return e.IsOrgMemberFn(agentID, orgID)
}

func (e OrgEnv) IsOrgAdmin(agentID string, orgID string) bool {
	if e.IsOrgAdminFn == nil {
		return false
	}
	return e.IsOrgAdminFn(agentID, orgID)
}

func (e OrgEnv) AuditOrgEvent(nowTick uint64, actorID string, action string, reason string, details map[string]any) {
	if e.AuditOrgEventFn != nil {
		e.AuditOrgEventFn(nowTick, actorID, action, reason, details)
	}
}
