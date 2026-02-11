package governance

import "strings"

type LawStatus string

const (
	LawNotice   LawStatus = "NOTICE"
	LawVoting   LawStatus = "VOTING"
	LawActive   LawStatus = "ACTIVE"
	LawRejected LawStatus = "REJECTED"
)

type Law struct {
	LawID      string
	LandID     string
	TemplateID string
	Title      string

	Params map[string]string // canonical string form

	ProposedBy     string
	ProposedTick   uint64
	NoticeEndsTick uint64
	VoteEndsTick   uint64

	Status LawStatus
	Votes  map[string]string // agent_id -> "YES"/"NO"/"ABSTAIN"
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
