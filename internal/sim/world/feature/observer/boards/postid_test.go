package boards

import "testing"

func TestNewPostID(t *testing.T) {
	if got := NewPostID(42); got != "P000042" {
		t.Fatalf("NewPostID mismatch: got %q want P000042", got)
	}
}
