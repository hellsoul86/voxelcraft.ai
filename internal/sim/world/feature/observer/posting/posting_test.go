package posting

import "testing"

func TestResolveBoardID(t *testing.T) {
	if got := ResolveBoardID("board_a", ""); got != "board_a" {
		t.Fatalf("expected board_a, got %q", got)
	}
	if got := ResolveBoardID("board_a", "board_b"); got != "board_b" {
		t.Fatalf("expected target override board_b, got %q", got)
	}
	if got := ResolveBoardID("  board_a ", " "); got != "board_a" {
		t.Fatalf("expected trimmed board_a, got %q", got)
	}
}

func TestValidatePostInput(t *testing.T) {
	if ok, _, _ := ValidatePostInput("", "t", "b"); ok {
		t.Fatalf("expected missing board validation failure")
	}
	if ok, _, _ := ValidatePostInput("board", "", "b"); ok {
		t.Fatalf("expected missing title validation failure")
	}
	if ok, _, _ := ValidatePostInput("board", "t", ""); ok {
		t.Fatalf("expected missing body validation failure")
	}
	if ok, _, _ := ValidatePostInput("board", string(make([]byte, 81)), "b"); ok {
		t.Fatalf("expected title length failure")
	}
	if ok, _, _ := ValidatePostInput("board", "t", string(make([]byte, 2001))); ok {
		t.Fatalf("expected body length failure")
	}
	if ok, code, msg := ValidatePostInput("board", "title", "body"); !ok || code != "" || msg != "" {
		t.Fatalf("expected valid post input, got ok=%v code=%q msg=%q", ok, code, msg)
	}
}

func TestValidateSearchInput(t *testing.T) {
	if ok, _, _ := ValidateSearchInput("", "query"); ok {
		t.Fatalf("expected missing board failure")
	}
	if ok, _, _ := ValidateSearchInput("board", ""); ok {
		t.Fatalf("expected missing query failure")
	}
	if ok, _, _ := ValidateSearchInput("board", string(make([]byte, 121))); ok {
		t.Fatalf("expected query length failure")
	}
	if ok, code, msg := ValidateSearchInput("board", "iron ingot"); !ok || code != "" || msg != "" {
		t.Fatalf("expected valid search input, got ok=%v code=%q msg=%q", ok, code, msg)
	}
}
