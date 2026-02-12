package interact

import "testing"

func TestBuildBoardPostsLimit(t *testing.T) {
	posts := []BoardPost{
		{PostID: "P1", Title: "1"},
		{PostID: "P2", Title: "2"},
		{PostID: "P3", Title: "3"},
	}
	out := BuildBoardPosts(posts, 2)
	if len(out) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(out))
	}
	if out[0]["post_id"] != "P2" || out[1]["post_id"] != "P3" {
		t.Fatalf("unexpected truncation: %#v", out)
	}
}

func TestValidateTransferNoop(t *testing.T) {
	ok, code, _ := ValidateTransferNoop("SELF", "SELF")
	if ok || code != "E_BAD_REQUEST" {
		t.Fatalf("expected no-op transfer failure")
	}
}
