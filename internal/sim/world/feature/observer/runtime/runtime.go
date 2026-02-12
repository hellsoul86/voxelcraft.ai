package runtime

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
)

type BoardPostInput struct {
	PostID string
	Author string
	Title  string
	Body   string
}

type BoardInput struct {
	BoardID string
	Posts   []BoardPostInput
}

func BuildStatus(hunger int, staminaMilli int, weather string) []string {
	status := make([]string, 0, 4)
	if hunger == 0 {
		status = append(status, "STARVING")
	} else if hunger < 5 {
		status = append(status, "HUNGRY")
	}
	if staminaMilli < 200 {
		status = append(status, "TIRED")
	}
	if weather == "STORM" {
		status = append(status, "STORM")
	} else if weather == "COLD" {
		status = append(status, "COLD")
	}
	if len(status) == 0 {
		status = append(status, "NONE")
	}
	return status
}

func BuildPublicBoards(inputs []BoardInput, maxPosts int, maxSummaryLen int) []protocol.BoardObs {
	if maxPosts <= 0 {
		maxPosts = 5
	}
	if maxSummaryLen <= 0 {
		maxSummaryLen = 120
	}
	if len(inputs) == 0 {
		return nil
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].BoardID < inputs[j].BoardID })
	out := make([]protocol.BoardObs, 0, len(inputs))
	for _, in := range inputs {
		if in.BoardID == "" || len(in.Posts) == 0 {
			continue
		}
		top := make([]protocol.BoardPost, 0, maxPosts)
		for i := len(in.Posts) - 1; i >= 0 && len(top) < maxPosts; i-- {
			p := in.Posts[i]
			summary := p.Body
			if len(summary) > maxSummaryLen {
				summary = summary[:maxSummaryLen]
			}
			top = append(top, protocol.BoardPost{
				PostID:  p.PostID,
				Author:  p.Author,
				Title:   p.Title,
				Summary: summary,
			})
		}
		out = append(out, protocol.BoardObs{BoardID: in.BoardID, TopPosts: top})
	}
	return out
}
