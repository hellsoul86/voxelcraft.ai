package runtime

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type PublicBoardsFromWorldInput struct {
	Boards map[string]*modelpkg.Board
	Self   modelpkg.Vec3i

	ParseContainerID func(id string) (typ string, pos modelpkg.Vec3i, ok bool)
	Distance         func(a, b modelpkg.Vec3i) int

	MaxDistance   int
	MaxPosts      int
	MaxSummaryLen int
}

func BuildPublicBoardsFromWorld(in PublicBoardsFromWorldInput) []protocol.BoardObs {
	if len(in.Boards) == 0 {
		return nil
	}
	maxDistance := in.MaxDistance
	if maxDistance <= 0 {
		maxDistance = 32
	}
	boardIDs := make([]string, 0, len(in.Boards))
	for id := range in.Boards {
		// For physical boards, only include nearby boards in OBS to keep payloads small.
		if in.ParseContainerID != nil && in.Distance != nil {
			if typ, pos, ok := in.ParseContainerID(id); ok && typ == "BULLETIN_BOARD" {
				if in.Distance(pos, in.Self) > maxDistance {
					continue
				}
			}
		}
		boardIDs = append(boardIDs, id)
	}
	sort.Strings(boardIDs)

	inputs := make([]BoardInput, 0, len(boardIDs))
	for _, bid := range boardIDs {
		b := in.Boards[bid]
		if b == nil || len(b.Posts) == 0 {
			continue
		}
		posts := make([]BoardPostInput, 0, len(b.Posts))
		for i := 0; i < len(b.Posts); i++ {
			p := b.Posts[i]
			posts = append(posts, BoardPostInput{
				PostID: p.PostID,
				Author: p.Author,
				Title:  p.Title,
				Body:   p.Body,
			})
		}
		inputs = append(inputs, BoardInput{
			BoardID: bid,
			Posts:   posts,
		})
	}
	return BuildPublicBoards(inputs, in.MaxPosts, in.MaxSummaryLen)
}
