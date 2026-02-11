package contracts

import (
	"sort"
	"strings"
)

type SummaryInput struct {
	ContractID   string
	State        string
	Kind         string
	Poster       string
	Acceptor     string
	DeadlineTick uint64
}

func NormalizeKind(k string) string {
	k = strings.TrimSpace(strings.ToUpper(k))
	switch k {
	case "GATHER", "DELIVER", "BUILD":
		return k
	default:
		return ""
	}
}

func BuildSummaries(in []SummaryInput) []map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	sort.Slice(in, func(i, j int) bool {
		return in[i].ContractID < in[j].ContractID
	})
	out := make([]map[string]interface{}, 0, len(in))
	for _, c := range in {
		out = append(out, map[string]interface{}{
			"contract_id":   c.ContractID,
			"state":         c.State,
			"kind":          c.Kind,
			"poster":        c.Poster,
			"acceptor":      c.Acceptor,
			"deadline_tick": c.DeadlineTick,
		})
	}
	return out
}
