package world

import (
	"fmt"
	"strings"

	"voxelcraft.ai/internal/protocol"
)

func handleInstantSetPermissions(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.LandID == "" || inst.Policy == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/policy"))
		return
	}
	land := w.claims[inst.LandID]
	if land == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !w.isLandAdmin(a.ID, land) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not land admin"))
		return
	}
	if v, ok := inst.Policy["allow_build"]; ok {
		land.Flags.AllowBuild = v
	}
	if v, ok := inst.Policy["allow_break"]; ok {
		land.Flags.AllowBreak = v
	}
	if v, ok := inst.Policy["allow_damage"]; ok {
		land.Flags.AllowDamage = v
	}
	if v, ok := inst.Policy["allow_trade"]; ok {
		land.Flags.AllowTrade = v
	}
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantUpgradeClaim(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.LandID == "" || inst.Radius <= 0 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/radius"))
		return
	}
	land := w.claims[inst.LandID]
	if land == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !w.isLandAdmin(a.ID, land) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not land admin"))
		return
	}
	if land.MaintenanceStage >= 1 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "land maintenance stage disallows expansion"))
		return
	}
	target := inst.Radius
	if target != 64 && target != 128 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "radius must be 64 or 128"))
		return
	}
	if target <= land.Radius {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "radius must increase"))
		return
	}
	if w.blockName(w.chunks.GetBlock(land.Anchor)) != "CLAIM_TOTEM" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "claim totem missing"))
		return
	}

	cost := map[string]int{}
	addCost := func(item string, n int) {
		if item == "" || n <= 0 {
			return
		}
		cost[item] += n
	}
	if land.Radius < 64 && target >= 64 {
		addCost("BATTERY", 1)
		addCost("CRYSTAL_SHARD", 2)
	}
	if land.Radius < 128 && target >= 128 {
		addCost("BATTERY", 2)
		addCost("CRYSTAL_SHARD", 4)
	}
	if len(cost) == 0 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "no upgrade needed"))
		return
	}
	for item, n := range cost {
		if a.Inventory[item] < n {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_RESOURCE", "missing upgrade materials"))
			return
		}
	}

	for _, c := range w.claims {
		if c == nil || c.LandID == land.LandID {
			continue
		}
		if abs(land.Anchor.X-c.Anchor.X) <= target+c.Radius && abs(land.Anchor.Z-c.Anchor.Z) <= target+c.Radius {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "claim overlaps existing land"))
			return
		}
	}

	for item, n := range cost {
		a.Inventory[item] -= n
		if a.Inventory[item] <= 0 {
			delete(a.Inventory, item)
		}
	}
	from := land.Radius
	land.Radius = target
	w.auditEvent(nowTick, a.ID, "CLAIM_UPGRADE", land.Anchor, "UPGRADE_CLAIM", map[string]any{
		"land_id": inst.LandID,
		"from":    from,
		"to":      target,
		"cost":    encodeItemPairs(cost),
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "land_id": inst.LandID, "radius": target})
}

func handleInstantAddMember(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.LandID == "" || inst.MemberID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/member_id"))
		return
	}
	land := w.claims[inst.LandID]
	if land == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !w.isLandAdmin(a.ID, land) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not land admin"))
		return
	}
	if land.Members == nil {
		land.Members = map[string]bool{}
	}
	land.Members[inst.MemberID] = true
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantRemoveMember(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.LandID == "" || inst.MemberID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/member_id"))
		return
	}
	land := w.claims[inst.LandID]
	if land == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !w.isLandAdmin(a.ID, land) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not land admin"))
		return
	}
	if land.Members != nil {
		delete(land.Members, inst.MemberID)
	}
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

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

func handleInstantDeedLand(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if inst.LandID == "" || inst.NewOwner == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/new_owner"))
		return
	}
	land := w.claims[inst.LandID]
	if land == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !w.isLandAdmin(a.ID, land) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not land admin"))
		return
	}
	newOwner := strings.TrimSpace(inst.NewOwner)
	if newOwner == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad new_owner"))
		return
	}
	if w.agents[newOwner] == nil && w.orgByID(newOwner) == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "new owner not found"))
		return
	}
	land.Owner = newOwner
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantProposeLaw(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if !w.cfg.AllowLaws {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "laws disabled in this world"))
		return
	}
	if inst.LandID == "" || inst.TemplateID == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing land_id/template_id"))
		return
	}
	land := w.claims[inst.LandID]
	if land == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !w.isLandMember(a.ID, land) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not eligible"))
		return
	}
	if _, ok := w.catalogs.Laws.ByID[inst.TemplateID]; !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "unknown law template"))
		return
	}
	if inst.Params == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing params"))
		return
	}

	params := map[string]string{}
	switch inst.TemplateID {
	case "MARKET_TAX":
		f, err := paramFloat(inst.Params, "market_tax")
		if err != nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
			return
		}
		if f < 0 {
			f = 0
		}
		if f > 0.25 {
			f = 0.25
		}
		params["market_tax"] = floatToCanonString(f)
	case "CURFEW_NO_BUILD":
		s, err := paramFloat(inst.Params, "start_time")
		if err != nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
			return
		}
		en, err := paramFloat(inst.Params, "end_time")
		if err != nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
			return
		}
		if s < 0 {
			s = 0
		}
		if s > 1 {
			s = 1
		}
		if en < 0 {
			en = 0
		}
		if en > 1 {
			en = 1
		}
		params["start_time"] = floatToCanonString(s)
		params["end_time"] = floatToCanonString(en)
	case "FINE_BREAK_PER_BLOCK":
		item, err := paramString(inst.Params, "fine_item")
		if err != nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
			return
		}
		if _, ok := w.catalogs.Items.Defs[item]; !ok {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "unknown fine_item"))
			return
		}
		n, err := paramInt(inst.Params, "fine_per_block")
		if err != nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
			return
		}
		if n < 0 {
			n = 0
		}
		if n > 100 {
			n = 100
		}
		params["fine_item"] = item
		params["fine_per_block"] = fmt.Sprintf("%d", n)
	case "ACCESS_PASS_CORE":
		item, err := paramString(inst.Params, "ticket_item")
		if err != nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
			return
		}
		if _, ok := w.catalogs.Items.Defs[item]; !ok {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "unknown ticket_item"))
			return
		}
		n, err := paramInt(inst.Params, "ticket_cost")
		if err != nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
			return
		}
		if n < 0 {
			n = 0
		}
		if n > 64 {
			n = 64
		}
		params["ticket_item"] = item
		params["ticket_cost"] = fmt.Sprintf("%d", n)
	default:
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "unsupported template"))
		return
	}

	tmpl := w.catalogs.Laws.ByID[inst.TemplateID]
	title := strings.TrimSpace(inst.Title)
	if title == "" {
		title = tmpl.Title
	}
	lawID := w.newLawID()
	notice := uint64(w.cfg.LawNoticeTicks)
	vote := uint64(w.cfg.LawVoteTicks)
	law := &Law{
		LawID:          lawID,
		LandID:         land.LandID,
		TemplateID:     inst.TemplateID,
		Title:          title,
		Params:         params,
		ProposedBy:     a.ID,
		ProposedTick:   nowTick,
		NoticeEndsTick: nowTick + notice,
		VoteEndsTick:   nowTick + notice + vote,
		Status:         LawNotice,
		Votes:          map[string]string{},
	}
	w.laws[lawID] = law
	w.broadcastLawEvent(nowTick, "PROPOSED", law, "")
	w.auditEvent(nowTick, a.ID, "LAW_PROPOSE", land.Anchor, "PROPOSE_LAW", map[string]any{
		"law_id":        lawID,
		"land_id":       land.LandID,
		"template_id":   inst.TemplateID,
		"title":         title,
		"notice_ends":   law.NoticeEndsTick,
		"vote_ends":     law.VoteEndsTick,
		"params":        law.Params,
		"proposed_by":   a.ID,
		"proposed_tick": nowTick,
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "law_id": lawID})
	if w.activeEventID == "CIVIC_VOTE" && nowTick < w.activeEventEnds {
		w.funOnWorldEventParticipation(a, w.activeEventID, nowTick)
		w.addFun(a, nowTick, "NARRATIVE", "civic_vote_propose", w.funDecay(a, "narrative:civic_vote_propose", 6, nowTick))
		a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "PROPOSE_LAW", "law_id": lawID})
	}
}

func handleInstantVote(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if !w.cfg.AllowLaws {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "laws disabled in this world"))
		return
	}
	if inst.LawID == "" || inst.Choice == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing law_id/choice"))
		return
	}
	law := w.laws[inst.LawID]
	if law == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "law not found"))
		return
	}
	if law.Status != LawVoting {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "law not in voting"))
		return
	}
	land := w.claims[law.LandID]
	if land == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !w.isLandMember(a.ID, land) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "not eligible to vote"))
		return
	}
	choice := strings.ToUpper(strings.TrimSpace(inst.Choice))
	switch choice {
	case "YES", "Y", "1", "TRUE":
		choice = "YES"
	case "NO", "N", "0", "FALSE":
		choice = "NO"
	case "ABSTAIN":
		choice = "ABSTAIN"
	default:
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad choice"))
		return
	}
	if law.Votes == nil {
		law.Votes = map[string]string{}
	}
	law.Votes[a.ID] = choice
	w.funOnVote(a, nowTick)
	w.auditEvent(nowTick, a.ID, "LAW_VOTE", land.Anchor, "VOTE", map[string]any{
		"law_id":   law.LawID,
		"land_id":  law.LandID,
		"choice":   choice,
		"voter_id": a.ID,
	})
	if w.activeEventID == "CIVIC_VOTE" && nowTick < w.activeEventEnds {
		a.AddEvent(protocol.Event{"t": nowTick, "type": "EVENT_GOAL", "event_id": w.activeEventID, "kind": "VOTE", "law_id": law.LawID})
	}
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}
