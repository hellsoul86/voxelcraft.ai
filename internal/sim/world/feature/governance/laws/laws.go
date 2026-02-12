package laws

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"voxelcraft.ai/internal/sim/world/feature/governance/claims"
)

var ErrUnsupportedLawTemplate = errors.New("unsupported template")

type Status string

const (
	StatusNotice   Status = "NOTICE"
	StatusVoting   Status = "VOTING"
	StatusActive   Status = "ACTIVE"
	StatusRejected Status = "REJECTED"
)

type Law struct {
	LawID      string
	LandID     string
	TemplateID string
	Title      string

	Params map[string]string

	ProposedBy     string
	ProposedTick   uint64
	NoticeEndsTick uint64
	VoteEndsTick   uint64

	Status Status
	Votes  map[string]string
}

func CountVotes(votes map[string]string) (yes, no int) {
	for _, v := range votes {
		switch NormalizeVoteChoice(v) {
		case "YES":
			yes++
		case "NO":
			no++
		}
	}
	return yes, no
}

func NormalizeVoteChoice(choice string) string {
	switch strings.ToUpper(strings.TrimSpace(choice)) {
	case "YES", "Y", "1", "TRUE":
		return "YES"
	case "NO", "N", "0", "FALSE":
		return "NO"
	case "ABSTAIN":
		return "ABSTAIN"
	default:
		return ""
	}
}

func ValidateProposeInput(allowLaws bool, landID, templateID string, params map[string]interface{}) (ok bool, code string, msg string) {
	if !allowLaws {
		return false, "E_NO_PERMISSION", "laws disabled in this world"
	}
	if strings.TrimSpace(landID) == "" || strings.TrimSpace(templateID) == "" {
		return false, "E_BAD_REQUEST", "missing land_id/template_id"
	}
	if params == nil {
		return false, "E_BAD_REQUEST", "missing params"
	}
	return true, "", ""
}

func ResolveLawTitle(provided, fallback string) string {
	title := strings.TrimSpace(provided)
	if title == "" {
		return fallback
	}
	return title
}

type Timeline struct {
	NoticeEnds uint64
	VoteEnds   uint64
}

func BuildLawTimeline(nowTick uint64, noticeTicks int, voteTicks int) Timeline {
	notice := uint64(maxInt(noticeTicks, 0))
	vote := uint64(maxInt(voteTicks, 0))
	return Timeline{
		NoticeEnds: nowTick + notice,
		VoteEnds:   nowTick + notice + vote,
	}
}

func ValidateVoteInput(allowLaws bool, lawID, choice string) (ok bool, code string, msg string) {
	if !allowLaws {
		return false, "E_NO_PERMISSION", "laws disabled in this world"
	}
	if strings.TrimSpace(lawID) == "" || strings.TrimSpace(choice) == "" {
		return false, "E_BAD_REQUEST", "missing law_id/choice"
	}
	return true, "", ""
}

type LandState struct {
	MarketTax         float64
	CurfewEnabled     bool
	CurfewStart       float64
	CurfewEnd         float64
	FineBreakEnabled  bool
	FineBreakItem     string
	FineBreakPerBlock int
	AccessPassEnabled bool
	AccessTicketItem  string
	AccessTicketCost  int
}

func NormalizeLawParams(templateID string, params map[string]interface{}, itemExists func(string) bool) (map[string]string, error) {
	switch templateID {
	case "MARKET_TAX":
		f, err := claims.ParamFloat(params, "market_tax")
		if err != nil {
			return nil, err
		}
		if f < 0 {
			f = 0
		}
		if f > 0.25 {
			f = 0.25
		}
		return map[string]string{"market_tax": claims.FloatToCanonString(f)}, nil
	case "CURFEW_NO_BUILD":
		s, err := claims.ParamFloat(params, "start_time")
		if err != nil {
			return nil, err
		}
		en, err := claims.ParamFloat(params, "end_time")
		if err != nil {
			return nil, err
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
		return map[string]string{
			"start_time": claims.FloatToCanonString(s),
			"end_time":   claims.FloatToCanonString(en),
		}, nil
	case "FINE_BREAK_PER_BLOCK":
		item, err := claims.ParamString(params, "fine_item")
		if err != nil {
			return nil, err
		}
		if itemExists != nil && !itemExists(item) {
			return nil, fmt.Errorf("unknown fine_item")
		}
		n, err := claims.ParamInt(params, "fine_per_block")
		if err != nil {
			return nil, err
		}
		if n < 0 {
			n = 0
		}
		if n > 100 {
			n = 100
		}
		return map[string]string{
			"fine_item":      item,
			"fine_per_block": fmt.Sprintf("%d", n),
		}, nil
	case "ACCESS_PASS_CORE":
		item, err := claims.ParamString(params, "ticket_item")
		if err != nil {
			return nil, err
		}
		if itemExists != nil && !itemExists(item) {
			return nil, fmt.Errorf("unknown ticket_item")
		}
		n, err := claims.ParamInt(params, "ticket_cost")
		if err != nil {
			return nil, err
		}
		if n < 0 {
			n = 0
		}
		if n > 64 {
			n = 64
		}
		return map[string]string{
			"ticket_item": item,
			"ticket_cost": fmt.Sprintf("%d", n),
		}, nil
	default:
		return nil, ErrUnsupportedLawTemplate
	}
}

func ApplyLawTemplate(templateID string, params map[string]string, in LandState) (LandState, error) {
	out := in
	switch templateID {
	case "MARKET_TAX":
		raw := params["market_tax"]
		if raw == "" {
			return in, fmt.Errorf("missing market_tax")
		}
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return in, fmt.Errorf("bad market_tax")
		}
		if f < 0 {
			f = 0
		}
		if f > 0.25 {
			f = 0.25
		}
		out.MarketTax = f
		return out, nil
	case "CURFEW_NO_BUILD":
		sRaw := params["start_time"]
		eRaw := params["end_time"]
		if sRaw == "" || eRaw == "" {
			return in, fmt.Errorf("missing start_time/end_time")
		}
		s, err := strconv.ParseFloat(sRaw, 64)
		if err != nil {
			return in, fmt.Errorf("bad start_time")
		}
		en, err := strconv.ParseFloat(eRaw, 64)
		if err != nil {
			return in, fmt.Errorf("bad end_time")
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
			out.CurfewEnabled = false
			out.CurfewStart = 0
			out.CurfewEnd = 0
			return out, nil
		}
		out.CurfewEnabled = true
		out.CurfewStart = s
		out.CurfewEnd = en
		return out, nil
	case "FINE_BREAK_PER_BLOCK":
		item := strings.TrimSpace(params["fine_item"])
		raw := strings.TrimSpace(params["fine_per_block"])
		if item == "" || raw == "" {
			out.FineBreakEnabled = false
			out.FineBreakItem = ""
			out.FineBreakPerBlock = 0
			return in, fmt.Errorf("missing fine_item/fine_per_block")
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			return in, fmt.Errorf("bad fine_per_block")
		}
		if n < 0 {
			n = 0
		}
		if n > 100 {
			n = 100
		}
		if n == 0 {
			out.FineBreakEnabled = false
			out.FineBreakItem = ""
			out.FineBreakPerBlock = 0
			return out, nil
		}
		out.FineBreakEnabled = true
		out.FineBreakItem = item
		out.FineBreakPerBlock = n
		return out, nil
	case "ACCESS_PASS_CORE":
		item := strings.TrimSpace(params["ticket_item"])
		raw := strings.TrimSpace(params["ticket_cost"])
		if item == "" || raw == "" {
			out.AccessPassEnabled = false
			out.AccessTicketItem = ""
			out.AccessTicketCost = 0
			return in, fmt.Errorf("missing ticket_item/ticket_cost")
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			return in, fmt.Errorf("bad ticket_cost")
		}
		if n < 0 {
			n = 0
		}
		if n > 64 {
			n = 64
		}
		if n == 0 {
			out.AccessPassEnabled = false
			out.AccessTicketItem = ""
			out.AccessTicketCost = 0
			return out, nil
		}
		out.AccessPassEnabled = true
		out.AccessTicketItem = item
		out.AccessTicketCost = n
		return out, nil
	default:
		return in, ErrUnsupportedLawTemplate
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
