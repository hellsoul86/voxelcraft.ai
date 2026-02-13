package runtime

import (
	"testing"

	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

func TestBuildPublicBoardsFromWorld_FilterAndSort(t *testing.T) {
	boards := map[string]*modelpkg.Board{
		"BULLETIN_BOARD@far": {
			BoardID: "BULLETIN_BOARD@far",
			Posts:   []modelpkg.BoardPost{{PostID: "P2", Author: "A", Title: "far", Body: "far"}},
		},
		"BULLETIN_BOARD@near": {
			BoardID: "BULLETIN_BOARD@near",
			Posts:   []modelpkg.BoardPost{{PostID: "P1", Author: "A", Title: "near", Body: "near"}},
		},
		"BOARD_GLOBAL": {
			BoardID: "BOARD_GLOBAL",
			Posts:   []modelpkg.BoardPost{{PostID: "P3", Author: "A", Title: "global", Body: "global"}},
		},
	}

	parse := func(id string) (string, modelpkg.Vec3i, bool) {
		switch id {
		case "BULLETIN_BOARD@near":
			return "BULLETIN_BOARD", modelpkg.Vec3i{X: 1, Y: 0, Z: 1}, true
		case "BULLETIN_BOARD@far":
			return "BULLETIN_BOARD", modelpkg.Vec3i{X: 100, Y: 0, Z: 100}, true
		default:
			return "", modelpkg.Vec3i{}, false
		}
	}
	dist := func(a, b modelpkg.Vec3i) int {
		dx := a.X - b.X
		if dx < 0 {
			dx = -dx
		}
		dz := a.Z - b.Z
		if dz < 0 {
			dz = -dz
		}
		return dx + dz
	}

	out := BuildPublicBoardsFromWorld(PublicBoardsFromWorldInput{
		Boards:           boards,
		Self:             modelpkg.Vec3i{X: 0, Y: 0, Z: 0},
		ParseContainerID: parse,
		Distance:         dist,
		MaxDistance:      32,
		MaxPosts:         5,
		MaxSummaryLen:    120,
	})

	if len(out) != 2 {
		t.Fatalf("expected 2 boards (near+global), got %d", len(out))
	}
	if out[0].BoardID != "BOARD_GLOBAL" || out[1].BoardID != "BULLETIN_BOARD@near" {
		t.Fatalf("unexpected board order: %+v", []string{out[0].BoardID, out[1].BoardID})
	}
}
