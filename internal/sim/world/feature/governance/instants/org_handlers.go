package instants

import (
	"voxelcraft.ai/internal/protocol"
	orgspkg "voxelcraft.ai/internal/sim/world/feature/governance/orgs"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type OrgActionResultFn func(tick uint64, ref string, ok bool, code string, message string) protocol.Event

type OrgInstantEnv interface {
	NewOrgID() string
	GetOrg(orgID string) *modelpkg.Organization
	PutOrg(org *modelpkg.Organization)
	DeleteOrg(orgID string)
	OrgTreasury(org *modelpkg.Organization) map[string]int
	IsOrgMember(agentID string, orgID string) bool
	IsOrgAdmin(agentID string, orgID string) bool
	AuditOrgEvent(nowTick uint64, actorID string, action string, reason string, details map[string]any)
}

func HandleCreateOrg(env OrgInstantEnv, ar OrgActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "org env unavailable"))
		return
	}
	kind := orgspkg.NormalizeOrgKind(inst.OrgKind)
	var k modelpkg.OrgKind
	switch kind {
	case orgspkg.KindGuild:
		k = modelpkg.OrgGuild
	case orgspkg.KindCity:
		k = modelpkg.OrgCity
	default:
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad org_kind"))
		return
	}
	if !orgspkg.ValidateOrgName(inst.OrgName) {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad org_name"))
		return
	}
	name := orgspkg.NormalizeOrgName(inst.OrgName)
	if a.OrgID != "" {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_CONFLICT", "already in org"))
		return
	}
	orgID := env.NewOrgID()
	org := &modelpkg.Organization{
		OrgID:       orgID,
		Kind:        k,
		Name:        name,
		CreatedTick: nowTick,
		MetaVersion: 1,
		Members:     map[string]modelpkg.OrgRole{a.ID: modelpkg.OrgLeader},
		Treasury:    map[string]int{},
	}
	env.PutOrg(org)
	a.OrgID = orgID
	env.AuditOrgEvent(nowTick, a.ID, "ORG_CREATE", "CREATE_ORG", map[string]any{
		"org_id":   orgID,
		"org_kind": string(k),
		"org_name": name,
		"leader":   a.ID,
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "org_id": orgID})
}

func HandleJoinOrg(env OrgInstantEnv, ar OrgActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "org env unavailable"))
		return
	}
	if inst.OrgID == "" {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing org_id"))
		return
	}
	org := env.GetOrg(inst.OrgID)
	if org == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "org not found"))
		return
	}
	if a.OrgID != "" {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_CONFLICT", "already in org"))
		return
	}
	if org.Members == nil {
		org.Members = map[string]modelpkg.OrgRole{}
	}
	org.Members[a.ID] = modelpkg.OrgMember
	org.MetaVersion++
	a.OrgID = org.OrgID
	env.AuditOrgEvent(nowTick, a.ID, "ORG_JOIN", "JOIN_ORG", map[string]any{
		"org_id":   org.OrgID,
		"member":   a.ID,
		"org_kind": string(org.Kind),
	})
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}

func HandleOrgDeposit(env OrgInstantEnv, ar OrgActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "org env unavailable"))
		return
	}
	if ok, code, msg := orgspkg.ValidateOrgTransferInput(inst.OrgID, inst.ItemID, inst.Count); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	org := env.GetOrg(inst.OrgID)
	if org == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "org not found"))
		return
	}
	if !env.IsOrgMember(a.ID, org.OrgID) {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_PERMISSION", "not org member"))
		return
	}
	if a.Inventory[inst.ItemID] < inst.Count {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_RESOURCE", "missing items"))
		return
	}
	a.Inventory[inst.ItemID] -= inst.Count
	if a.Inventory[inst.ItemID] <= 0 {
		delete(a.Inventory, inst.ItemID)
	}
	tr := env.OrgTreasury(org)
	tr[inst.ItemID] += inst.Count
	env.AuditOrgEvent(nowTick, a.ID, "ORG_DEPOSIT", "ORG_DEPOSIT", map[string]any{
		"org_id": org.OrgID,
		"item":   inst.ItemID,
		"count":  inst.Count,
	})
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}

func HandleOrgWithdraw(env OrgInstantEnv, ar OrgActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "org env unavailable"))
		return
	}
	if ok, code, msg := orgspkg.ValidateOrgTransferInput(inst.OrgID, inst.ItemID, inst.Count); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	org := env.GetOrg(inst.OrgID)
	if org == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "org not found"))
		return
	}
	if !env.IsOrgAdmin(a.ID, org.OrgID) {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_PERMISSION", "not org admin"))
		return
	}
	tr := env.OrgTreasury(org)
	if tr[inst.ItemID] < inst.Count {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_RESOURCE", "treasury lacks items"))
		return
	}
	tr[inst.ItemID] -= inst.Count
	if tr[inst.ItemID] <= 0 {
		delete(tr, inst.ItemID)
	}
	a.Inventory[inst.ItemID] += inst.Count
	env.AuditOrgEvent(nowTick, a.ID, "ORG_WITHDRAW", "ORG_WITHDRAW", map[string]any{
		"org_id": org.OrgID,
		"item":   inst.ItemID,
		"count":  inst.Count,
	})
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}

func HandleLeaveOrg(env OrgInstantEnv, ar OrgActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "org env unavailable"))
		return
	}
	if a.OrgID == "" {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BLOCKED", "not in org"))
		return
	}
	org := env.GetOrg(a.OrgID)
	orgID := a.OrgID
	a.OrgID = ""
	if org == nil || org.Members == nil {
		a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
		return
	}
	role := org.Members[a.ID]
	delete(org.Members, a.ID)
	org.MetaVersion++
	if len(org.Members) == 0 {
		env.DeleteOrg(orgID)
		a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
		return
	}
	if role == modelpkg.OrgLeader {
		memberIDs := make([]string, 0, len(org.Members))
		for aid := range org.Members {
			memberIDs = append(memberIDs, aid)
		}
		best := orgspkg.SelectNextLeader(memberIDs)
		if best != "" {
			org.Members[best] = modelpkg.OrgLeader
			org.MetaVersion++
		}
	}
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}
