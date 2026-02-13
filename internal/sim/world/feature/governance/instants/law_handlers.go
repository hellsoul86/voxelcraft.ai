package instants

import (
	"errors"

	"voxelcraft.ai/internal/protocol"
	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type LawInstantEnv interface {
	GetLand(landID string) *modelpkg.LandClaim
	IsLandMember(agentID string, land *modelpkg.LandClaim) bool
	GetLawTemplateTitle(templateID string) (string, bool)
	ItemExists(itemID string) bool
	NewLawID() string
	PutLaw(law *lawspkg.Law)
	GetLaw(lawID string) *lawspkg.Law
	BroadcastLawEvent(nowTick uint64, stage string, law *lawspkg.Law, note string)
	AuditLawEvent(nowTick uint64, actorID string, action string, pos modelpkg.Vec3i, reason string, details map[string]any)
}

type LawInstantHooks struct {
	OnProposed func(law *lawspkg.Law)
	OnVoted    func(law *lawspkg.Law, choice string)
}

func HandleProposeLaw(env LawInstantEnv, ar OrgActionResultFn, hooks LawInstantHooks, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64, allowLaws bool, lawNoticeTicks int, lawVoteTicks int) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "law env unavailable"))
		return
	}
	if ok, code, msg := lawspkg.ValidateProposeInput(allowLaws, inst.LandID, inst.TemplateID, inst.Params); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	land := env.GetLand(inst.LandID)
	if land == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !env.IsLandMember(a.ID, land) {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_PERMISSION", "not eligible"))
		return
	}
	templateTitle, ok := env.GetLawTemplateTitle(inst.TemplateID)
	if !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "unknown law template"))
		return
	}
	params, err := lawspkg.NormalizeLawParams(inst.TemplateID, inst.Params, env.ItemExists)
	if err != nil {
		if errors.Is(err, lawspkg.ErrUnsupportedLawTemplate) {
			a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "unsupported template"))
			return
		}
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BAD_REQUEST", err.Error()))
		return
	}

	title := lawspkg.ResolveLawTitle(inst.Title, templateTitle)
	lawID := env.NewLawID()
	timeline := lawspkg.BuildLawTimeline(nowTick, lawNoticeTicks, lawVoteTicks)
	law := &lawspkg.Law{
		LawID:          lawID,
		LandID:         land.LandID,
		TemplateID:     inst.TemplateID,
		Title:          title,
		Params:         params,
		ProposedBy:     a.ID,
		ProposedTick:   nowTick,
		NoticeEndsTick: timeline.NoticeEnds,
		VoteEndsTick:   timeline.VoteEnds,
		Status:         lawspkg.StatusNotice,
		Votes:          map[string]string{},
	}
	env.PutLaw(law)
	env.BroadcastLawEvent(nowTick, "PROPOSED", law, "")
	env.AuditLawEvent(nowTick, a.ID, "LAW_PROPOSE", land.Anchor, "PROPOSE_LAW", map[string]any{
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
	if hooks.OnProposed != nil {
		hooks.OnProposed(law)
	}
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "law_id": lawID})
}

func HandleVoteLaw(env LawInstantEnv, ar OrgActionResultFn, hooks LawInstantHooks, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64, allowLaws bool) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "law env unavailable"))
		return
	}
	if ok, code, msg := lawspkg.ValidateVoteInput(allowLaws, inst.LawID, inst.Choice); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, msg))
		return
	}
	law := env.GetLaw(inst.LawID)
	if law == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "law not found"))
		return
	}
	if law.Status != lawspkg.StatusVoting {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BLOCKED", "law not in voting"))
		return
	}
	land := env.GetLand(law.LandID)
	if land == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "land not found"))
		return
	}
	if !env.IsLandMember(a.ID, land) {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_PERMISSION", "not eligible to vote"))
		return
	}
	choice := lawspkg.NormalizeVoteChoice(inst.Choice)
	if choice == "" {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BAD_REQUEST", "bad choice"))
		return
	}
	if law.Votes == nil {
		law.Votes = map[string]string{}
	}
	law.Votes[a.ID] = choice
	env.AuditLawEvent(nowTick, a.ID, "LAW_VOTE", land.Anchor, "VOTE", map[string]any{
		"law_id":   law.LawID,
		"land_id":  law.LandID,
		"choice":   choice,
		"voter_id": a.ID,
	})
	if hooks.OnVoted != nil {
		hooks.OnVoted(law, choice)
	}
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}
