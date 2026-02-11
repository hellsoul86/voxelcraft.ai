package session

import "sort"

type ResumeCandidate struct {
	ID          string
	ResumeToken string
}

func SortedIDs[T any](m map[string]T) []string {
	if len(m) == 0 {
		return nil
	}
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func FindResumeAgentID(candidates []ResumeCandidate, token string) string {
	if token == "" || len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	for _, c := range candidates {
		if c.ResumeToken == token {
			return c.ID
		}
	}
	return ""
}
