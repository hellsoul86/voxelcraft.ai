package world

import (
	"errors"

	"voxelcraft.ai/internal/protocol"
	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
)

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
