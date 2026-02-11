package world

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
)

func handleInstantCreateOrg(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	kind := strings.ToUpper(strings.TrimSpace(inst.OrgKind))
	var k OrgKind
	switch kind {
	case string(OrgGuild):
		k = OrgGuild
	case string(OrgCity):
		k = OrgCity
	default:
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad org_kind"))
		return
	}
	name := strings.TrimSpace(inst.OrgName)
	if name == "" || len(name) > 40 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad org_name"))
		return
	}
	if a.OrgID != "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "already in org"))
		return
	}
	orgID := w.newOrgID()
	w.orgs[orgID] = &Organization{
		OrgID:       orgID,
		Kind:        k,
		Name:        name,
		CreatedTick: nowTick,
		MetaVersion: 1,
		Members:     map[string]OrgRole{a.ID: OrgLeader},
		Treasury:    map[string]int{},
	}
	a.OrgID = orgID
	w.auditEvent(nowTick, a.ID, "ORG_CREATE", a.Pos, "CREATE_ORG", map[string]any{
		"org_id":   orgID,
		"org_kind": string(k),
		"org_name": name,
		"leader":   a.ID,
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "org_id": orgID})
}

func handleInstantJoinOrg(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.OrgID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing org_id"))
		return
	}
	org := w.orgByID(inst.OrgID)
	if org == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "org not found"))
		return
	}
	if a.OrgID != "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "already in org"))
		return
	}
	if org.Members == nil {
		org.Members = map[string]OrgRole{}
	}
	org.Members[a.ID] = OrgMember
	org.MetaVersion++
	a.OrgID = org.OrgID
	w.auditEvent(nowTick, a.ID, "ORG_JOIN", a.Pos, "JOIN_ORG", map[string]any{
		"org_id":   org.OrgID,
		"member":   a.ID,
		"org_kind": string(org.Kind),
	})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantOrgDeposit(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.OrgID == "" || inst.ItemID == "" || inst.Count <= 0 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing org_id/item_id/count"))
		return
	}
	org := w.orgByID(inst.OrgID)
	if org == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "org not found"))
		return
	}
	if !w.isOrgMember(a.ID, org.OrgID) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not org member"))
		return
	}
	if a.Inventory[inst.ItemID] < inst.Count {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "missing items"))
		return
	}
	a.Inventory[inst.ItemID] -= inst.Count
	if a.Inventory[inst.ItemID] <= 0 {
		delete(a.Inventory, inst.ItemID)
	}
	tr := w.orgTreasury(org)
	tr[inst.ItemID] += inst.Count
	w.auditEvent(nowTick, a.ID, "ORG_DEPOSIT", a.Pos, "ORG_DEPOSIT", map[string]any{
		"org_id": org.OrgID,
		"item":   inst.ItemID,
		"count":  inst.Count,
	})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantOrgWithdraw(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.OrgID == "" || inst.ItemID == "" || inst.Count <= 0 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing org_id/item_id/count"))
		return
	}
	org := w.orgByID(inst.OrgID)
	if org == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "org not found"))
		return
	}
	if !w.isOrgAdmin(a.ID, org.OrgID) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not org admin"))
		return
	}
	tr := w.orgTreasury(org)
	if tr[inst.ItemID] < inst.Count {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "treasury lacks items"))
		return
	}
	tr[inst.ItemID] -= inst.Count
	if tr[inst.ItemID] <= 0 {
		delete(tr, inst.ItemID)
	}
	a.Inventory[inst.ItemID] += inst.Count
	w.auditEvent(nowTick, a.ID, "ORG_WITHDRAW", a.Pos, "ORG_WITHDRAW", map[string]any{
		"org_id": org.OrgID,
		"item":   inst.ItemID,
		"count":  inst.Count,
	})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantLeaveOrg(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if a.OrgID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "not in org"))
		return
	}
	org := w.orgByID(a.OrgID)
	orgID := a.OrgID
	a.OrgID = ""
	if org == nil || org.Members == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
		return
	}
	role := org.Members[a.ID]
	delete(org.Members, a.ID)
	org.MetaVersion++
	if len(org.Members) == 0 {
		delete(w.orgs, orgID)
		a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
		return
	}
	if role == OrgLeader {
		best := ""
		for aid := range org.Members {
			if best == "" || aid < best {
				best = aid
			}
		}
		if best != "" {
			org.Members[best] = OrgLeader
			org.MetaVersion++
		}
	}
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}
