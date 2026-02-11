package world

import (
	"fmt"
	"strings"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/world/feature/governance"
)

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
		f, err := governance.ParamFloat(inst.Params, "market_tax")
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
		params["market_tax"] = governance.FloatToCanonString(f)
	case "CURFEW_NO_BUILD":
		s, err := governance.ParamFloat(inst.Params, "start_time")
		if err != nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
			return
		}
		en, err := governance.ParamFloat(inst.Params, "end_time")
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
		params["start_time"] = governance.FloatToCanonString(s)
		params["end_time"] = governance.FloatToCanonString(en)
	case "FINE_BREAK_PER_BLOCK":
		item, err := governance.ParamString(inst.Params, "fine_item")
		if err != nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
			return
		}
		if _, ok := w.catalogs.Items.Defs[item]; !ok {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "unknown fine_item"))
			return
		}
		n, err := governance.ParamInt(inst.Params, "fine_per_block")
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
		item, err := governance.ParamString(inst.Params, "ticket_item")
		if err != nil {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
			return
		}
		if _, ok := w.catalogs.Items.Defs[item]; !ok {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "unknown ticket_item"))
			return
		}
		n, err := governance.ParamInt(inst.Params, "ticket_cost")
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
	choice := governance.NormalizeVoteChoice(inst.Choice)
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
