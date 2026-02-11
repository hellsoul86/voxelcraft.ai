package world

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/world/feature/governance"
)

type LawStatus = governance.LawStatus

const (
	LawNotice   LawStatus = governance.LawNotice
	LawVoting   LawStatus = governance.LawVoting
	LawActive   LawStatus = governance.LawActive
	LawRejected LawStatus = governance.LawRejected
)

type Law = governance.Law

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
		switch law.Status {
		case LawNotice:
			if nowTick >= law.NoticeEndsTick {
				law.Status = LawVoting
				w.broadcastLawEvent(nowTick, "VOTING", law, "")
			}
		case LawVoting:
			if nowTick >= law.VoteEndsTick {
				yes, no := governance.CountVotes(law.Votes)
				if yes > no {
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
	switch law.TemplateID {
	case "MARKET_TAX":
		raw := law.Params["market_tax"]
		if raw == "" {
			return fmt.Errorf("missing market_tax")
		}
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("bad market_tax")
		}
		if f < 0 {
			f = 0
		}
		if f > 0.25 {
			f = 0.25
		}
		land.MarketTax = f
		return nil

	case "CURFEW_NO_BUILD":
		sRaw := law.Params["start_time"]
		eRaw := law.Params["end_time"]
		if sRaw == "" || eRaw == "" {
			return fmt.Errorf("missing start_time/end_time")
		}
		s, err := strconv.ParseFloat(sRaw, 64)
		if err != nil {
			return fmt.Errorf("bad start_time")
		}
		en, err := strconv.ParseFloat(eRaw, 64)
		if err != nil {
			return fmt.Errorf("bad end_time")
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
		if s == en {
			land.CurfewEnabled = false
			land.CurfewStart = 0
			land.CurfewEnd = 0
			return nil
		}
		land.CurfewEnabled = true
		land.CurfewStart = s
		land.CurfewEnd = en
		return nil

	case "FINE_BREAK_PER_BLOCK":
		item := strings.TrimSpace(law.Params["fine_item"])
		raw := strings.TrimSpace(law.Params["fine_per_block"])
		if item == "" || raw == "" {
			land.FineBreakEnabled = false
			land.FineBreakItem = ""
			land.FineBreakPerBlock = 0
			return fmt.Errorf("missing fine_item/fine_per_block")
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("bad fine_per_block")
		}
		if n < 0 {
			n = 0
		}
		if n > 100 {
			n = 100
		}
		if n == 0 {
			land.FineBreakEnabled = false
			land.FineBreakItem = ""
			land.FineBreakPerBlock = 0
			return nil
		}
		land.FineBreakEnabled = true
		land.FineBreakItem = item
		land.FineBreakPerBlock = n
		return nil

	case "ACCESS_PASS_CORE":
		item := strings.TrimSpace(law.Params["ticket_item"])
		raw := strings.TrimSpace(law.Params["ticket_cost"])
		if item == "" || raw == "" {
			land.AccessPassEnabled = false
			land.AccessTicketItem = ""
			land.AccessTicketCost = 0
			return fmt.Errorf("missing ticket_item/ticket_cost")
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("bad ticket_cost")
		}
		if n < 0 {
			n = 0
		}
		if n > 64 {
			n = 64
		}
		if n == 0 {
			land.AccessPassEnabled = false
			land.AccessTicketItem = ""
			land.AccessTicketCost = 0
			return nil
		}
		land.AccessPassEnabled = true
		land.AccessTicketItem = item
		land.AccessTicketCost = n
		return nil

	default:
		return fmt.Errorf("unsupported template")
	}
}
