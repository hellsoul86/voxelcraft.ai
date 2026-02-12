package runtime

import "testing"

func TestBuildStatus(t *testing.T) {
	got := BuildStatus(0, 100, "COLD")
	if len(got) == 0 {
		t.Fatalf("empty status")
	}
}

func TestBuildPublicBoards(t *testing.T) {
	boards := BuildPublicBoards([]BoardInput{
		{
			BoardID: "B1",
			Posts: []BoardPostInput{
				{PostID: "P1", Author: "A", Title: "T1", Body: "hello"},
				{PostID: "P2", Author: "B", Title: "T2", Body: "world"},
			},
		},
	}, 1, 10)
	if len(boards) != 1 || len(boards[0].TopPosts) != 1 || boards[0].TopPosts[0].PostID != "P2" {
		t.Fatalf("unexpected boards output: %+v", boards)
	}
}
