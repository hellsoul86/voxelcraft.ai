package search

import (
	"testing"

	boardspkg "voxelcraft.ai/internal/sim/world/feature/observer/boards"
)

func TestNormalizeBoardSearchLimit(t *testing.T) {
	if got := NormalizeBoardSearchLimit(0); got != 20 {
		t.Fatalf("default limit mismatch: got %d want 20", got)
	}
	if got := NormalizeBoardSearchLimit(100); got != 50 {
		t.Fatalf("max limit mismatch: got %d want 50", got)
	}
	if got := NormalizeBoardSearchLimit(7); got != 7 {
		t.Fatalf("pass-through limit mismatch: got %d want 7", got)
	}
}

func TestMatchBoardPosts(t *testing.T) {
	posts := []boardspkg.Post{
		{PostID: "P1", Author: "A", Title: "Sell Iron", Body: "10 ingots", Tick: 1},
		{PostID: "P2", Author: "B", Title: "Buy Plank", Body: "need wood", Tick: 2},
		{PostID: "P3", Author: "C", Title: "Sell Crystal", Body: "rare crystal", Tick: 3},
	}
	res := MatchBoardPosts(posts, "sell", 10)
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	if got := res[0]["post_id"]; got != "P3" {
		t.Fatalf("newest-first mismatch, got %v want P3", got)
	}
	if got := res[1]["post_id"]; got != "P1" {
		t.Fatalf("second result mismatch, got %v want P1", got)
	}
}
