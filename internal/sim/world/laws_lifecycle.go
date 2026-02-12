package world

import (
	"fmt"
	"sort"

	"voxelcraft.ai/internal/protocol"
	lawspkg "voxelcraft.ai/internal/sim/world/feature/governance/laws"
	lawsruntimepkg "voxelcraft.ai/internal/sim/world/feature/governance/laws/runtime"
)

type LawStatus = lawspkg.Status

const (
	LawNotice   LawStatus = lawspkg.StatusNotice
	LawVoting   LawStatus = lawspkg.StatusVoting
	LawActive   LawStatus = lawspkg.StatusActive
	LawRejected LawStatus = lawspkg.StatusRejected
)

type Law = lawspkg.Law

func (w *World) newLawID() string {
	n := w.nextLawNum.Add(1)
	return fmt.Sprintf("LAW%06d", n)
}

func (w *World) tickLaws(nowTick uint64) {
	if len(w.laws) == 0 {
		return
	}
	ids := make([]string, 0, len(w.laws))
	for id := range w.laws {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		law := w.laws[id]
		if law == nil {
			continue
		}
		tr := lawsruntimepkg.NextTransition(lawsruntimepkg.TransitionInput{
			Status:         string(law.Status),
			NowTick:        nowTick,
			NoticeEndsTick: law.NoticeEndsTick,
			VoteEndsTick:   law.VoteEndsTick,
		})
		if tr.ShouldTransition && tr.NextStatus == string(LawVoting) {
			law.Status = LawVoting
			w.broadcastLawEvent(nowTick, tr.EventKind, law, "")
			continue
		}
		if tr.ShouldTransition && law.Status == LawVoting {
			yes, no := lawspkg.CountVotes(law.Votes)
			if lawsruntimepkg.VotePassed(yes, no) {
				if err := w.activateLaw(nowTick, law); err != nil {
					law.Status = LawRejected
					if land := w.claims[law.LandID]; land != nil {
						w.auditEvent(nowTick, "WORLD", "LAW_REJECTED", land.Anchor, "ACTIVATE_FAILED", map[string]any{
							"law_id":      law.LawID,
							"land_id":     law.LandID,
							"template_id": law.TemplateID,
							"title":       law.Title,
							"yes":         yes,
							"no":          no,
							"message":     err.Error(),
						})
					}
					w.broadcastLawEvent(nowTick, "REJECTED", law, err.Error())
					continue
				}
				if proposer := w.agents[law.ProposedBy]; proposer != nil {
					w.funOnLawActive(proposer, nowTick)
				}
				law.Status = LawActive
				if land := w.claims[law.LandID]; land != nil {
					w.auditEvent(nowTick, "WORLD", "LAW_ACTIVE", land.Anchor, "VOTE_PASSED", map[string]any{
						"law_id":      law.LawID,
						"land_id":     law.LandID,
						"template_id": law.TemplateID,
						"title":       law.Title,
						"yes":         yes,
						"no":          no,
						"params":      law.Params,
					})
				}
				w.broadcastLawEvent(nowTick, "ACTIVE", law, "")
			} else {
				law.Status = LawRejected
				if land := w.claims[law.LandID]; land != nil {
					w.auditEvent(nowTick, "WORLD", "LAW_REJECTED", land.Anchor, "VOTE_FAILED", map[string]any{
						"law_id":      law.LawID,
						"land_id":     law.LandID,
						"template_id": law.TemplateID,
						"title":       law.Title,
						"yes":         yes,
						"no":          no,
					})
				}
				w.broadcastLawEvent(nowTick, "REJECTED", law, "vote failed")
			}
		}
	}
}

func (w *World) broadcastLawEvent(nowTick uint64, kind string, law *Law, message string) {
	base := protocol.Event{
		"t":           nowTick,
		"type":        "LAW",
		"kind":        kind,
		"law_id":      law.LawID,
		"land_id":     law.LandID,
		"template_id": law.TemplateID,
		"title":       law.Title,
		"status":      string(law.Status),
	}
	if message != "" {
		base["message"] = message
	}
	if kind == "PROPOSED" {
		base["notice_ends_tick"] = law.NoticeEndsTick
		base["vote_ends_tick"] = law.VoteEndsTick
	}
	for _, a := range w.agents {
		// Copy map per agent to avoid shared references.
		e := protocol.Event{}
		for k, v := range base {
			e[k] = v
		}
		a.AddEvent(e)
	}
}

func (w *World) activateLaw(nowTick uint64, law *Law) error {
	_ = nowTick
	land := w.claims[law.LandID]
	if land == nil {
		return fmt.Errorf("land not found")
	}
	in := lawspkg.LandState{
		MarketTax:         land.MarketTax,
		CurfewEnabled:     land.CurfewEnabled,
		CurfewStart:       land.CurfewStart,
		CurfewEnd:         land.CurfewEnd,
		FineBreakEnabled:  land.FineBreakEnabled,
		FineBreakItem:     land.FineBreakItem,
		FineBreakPerBlock: land.FineBreakPerBlock,
		AccessPassEnabled: land.AccessPassEnabled,
		AccessTicketItem:  land.AccessTicketItem,
		AccessTicketCost:  land.AccessTicketCost,
	}
	out, err := lawspkg.ApplyLawTemplate(law.TemplateID, law.Params, in)
	if err != nil {
		return err
	}
	land.MarketTax = out.MarketTax
	land.CurfewEnabled = out.CurfewEnabled
	land.CurfewStart = out.CurfewStart
	land.CurfewEnd = out.CurfewEnd
	land.FineBreakEnabled = out.FineBreakEnabled
	land.FineBreakItem = out.FineBreakItem
	land.FineBreakPerBlock = out.FineBreakPerBlock
	land.AccessPassEnabled = out.AccessPassEnabled
	land.AccessTicketItem = out.AccessTicketItem
	land.AccessTicketCost = out.AccessTicketCost
	return nil
}
