package search

import (
	"strings"

	boards "voxelcraft.ai/internal/sim/world/feature/observer/boards"
)

func NormalizeBoardSearchLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func MatchBoardPosts(posts []boards.Post, query string, limit int) []map[string]any {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" || len(posts) == 0 || limit <= 0 {
		return nil
	}
	results := make([]map[string]any, 0, limit)
	for i := len(posts) - 1; i >= 0 && len(results) < limit; i-- {
		p := posts[i]
		if strings.Contains(strings.ToLower(p.Title), q) || strings.Contains(strings.ToLower(p.Body), q) {
			body := p.Body
			if len(body) > 400 {
				body = body[:400]
			}
			results = append(results, map[string]any{
				"post_id": p.PostID,
				"author":  p.Author,
				"title":   p.Title,
				"body":    body,
				"tick":    p.Tick,
			})
		}
	}
	return results
}
