package world

import (
	"errors"

	"voxelcraft.ai/internal/protocol"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	claimspkg "voxelcraft.ai/internal/sim/world/feature/governance/claims"
	governanceinstantspkg "voxelcraft.ai/internal/sim/world/feature/governance/instants"
	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
	orgspkg "voxelcraft.ai/internal/sim/world/feature/governance/orgs"
)

func handleInstantSetPermissions(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := claimspkg.ValidateSetPermissionsInput(inst.LandID, inst.Policy); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	land := w.claims[inst.LandID]
	if ok, code, msg := governanceinstantspkg.ValidateLandAdmin(land != nil, w.isLandAdmin(a.ID, land)); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	next := claimspkg.ApplyPolicyFlags(claimspkg.Flags{
		AllowBuild:  land.Flags.AllowBuild,
		AllowBreak:  land.Flags.AllowBreak,
		AllowDamage: land.Flags.AllowDamage,
		AllowTrade:  land.Flags.AllowTrade,
	}, inst.Policy)
	land.Flags.AllowBuild = next.AllowBuild
	land.Flags.AllowBreak = next.AllowBreak
	land.Flags.AllowDamage = next.AllowDamage
	land.Flags.AllowTrade = next.AllowTrade
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantUpgradeClaim(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := claimspkg.ValidateUpgradeInput(inst.LandID, inst.Radius); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	land := w.claims[inst.LandID]
	if ok, code, msg := governanceinstantspkg.ValidateLandAdmin(land != nil, w.isLandAdmin(a.ID, land)); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	if land.MaintenanceStage >= 1 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "land maintenance stage disallows expansion"))
		return
	}
	target := inst.Radius
	if ok, code, msg := claimspkg.ValidateUpgradeRadius(land.Radius, target); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	if w.blockName(w.chunks.GetBlock(land.Anchor)) != "CLAIM_TOTEM" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "claim totem missing"))
		return
	}

	cost := claimspkg.UpgradeCost(land.Radius, target)
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

	records := make([]governanceinstantspkg.ClaimRecord, 0, len(w.claims))
	for _, c := range w.claims {
		if c == nil {
			continue
		}
		records = append(records, governanceinstantspkg.ClaimRecord{
			LandID:  c.LandID,
			AnchorX: c.Anchor.X,
			AnchorZ: c.Anchor.Z,
			Radius:  c.Radius,
		})
	}
	zones := governanceinstantspkg.BuildZones(records)
	if claimspkg.UpgradeOverlaps(land.Anchor.X, land.Anchor.Z, target, land.LandID, zones) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_CONFLICT", "claim overlaps existing land"))
		return
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
		"cost":    inventorypkg.EncodeItemPairs(cost),
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "land_id": inst.LandID, "radius": target})
}

func handleInstantAddMember(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := claimspkg.ValidateMemberMutationInput(inst.LandID, inst.MemberID); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	land := w.claims[inst.LandID]
	if ok, code, msg := governanceinstantspkg.ValidateLandAdmin(land != nil, w.isLandAdmin(a.ID, land)); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	if land.Members == nil {
		land.Members = map[string]bool{}
	}
	land.Members[inst.MemberID] = true
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantRemoveMember(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := claimspkg.ValidateMemberMutationInput(inst.LandID, inst.MemberID); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	land := w.claims[inst.LandID]
	if ok, code, msg := governanceinstantspkg.ValidateLandAdmin(land != nil, w.isLandAdmin(a.ID, land)); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	if land.Members != nil {
		delete(land.Members, inst.MemberID)
	}
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantDeedLand(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, code, msg := claimspkg.ValidateDeedInput(inst.LandID, inst.NewOwner); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	land := w.claims[inst.LandID]
	if ok, code, msg := governanceinstantspkg.ValidateLandAdmin(land != nil, w.isLandAdmin(a.ID, land)); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
		return
	}
	newOwner := claimspkg.NormalizeNewOwner(inst.NewOwner)
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
	if ok, code, msg := lawspkg.ValidateProposeInput(w.cfg.AllowLaws, inst.LandID, inst.TemplateID, inst.Params); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
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

	params, err := lawspkg.NormalizeLawParams(inst.TemplateID, inst.Params, func(item string) bool {
		_, ok := w.catalogs.Items.Defs[item]
		return ok
	})
	if err != nil {
		if errors.Is(err, lawspkg.ErrUnsupportedLawTemplate) {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "unsupported template"))
			return
		}
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
		return
	}

	tmpl := w.catalogs.Laws.ByID[inst.TemplateID]
	title := lawspkg.ResolveLawTitle(inst.Title, tmpl.Title)
	lawID := w.newLawID()
	timeline := lawspkg.BuildLawTimeline(nowTick, w.cfg.LawNoticeTicks, w.cfg.LawVoteTicks)
	law := &Law{
		LawID:          lawID,
		LandID:         land.LandID,
		TemplateID:     inst.TemplateID,
		Title:          title,
		Params:         params,
		ProposedBy:     a.ID,
		ProposedTick:   nowTick,
		NoticeEndsTick: timeline.NoticeEnds,
		VoteEndsTick:   timeline.VoteEnds,
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
	if ok, code, msg := lawspkg.ValidateVoteInput(w.cfg.AllowLaws, inst.LawID, inst.Choice); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
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
	choice := lawspkg.NormalizeVoteChoice(inst.Choice)
	if choice == "" {
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

func handleInstantCreateOrg(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	kind := orgspkg.NormalizeOrgKind(inst.OrgKind)
	var k OrgKind
	switch kind {
	case orgspkg.KindGuild:
		k = OrgGuild
	case orgspkg.KindCity:
		k = OrgCity
	default:
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad org_kind"))
		return
	}
	if !orgspkg.ValidateOrgName(inst.OrgName) {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad org_name"))
		return
	}
	name := orgspkg.NormalizeOrgName(inst.OrgName)
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
	if ok, code, msg := orgspkg.ValidateOrgTransferInput(inst.OrgID, inst.ItemID, inst.Count); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
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
	if ok, code, msg := orgspkg.ValidateOrgTransferInput(inst.OrgID, inst.ItemID, inst.Count); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, msg))
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
		memberIDs := make([]string, 0, len(org.Members))
		for aid := range org.Members {
			memberIDs = append(memberIDs, aid)
		}
		best := orgspkg.SelectNextLeader(memberIDs)
		if best != "" {
			org.Members[best] = OrgLeader
			org.MetaVersion++
		}
	}
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}
