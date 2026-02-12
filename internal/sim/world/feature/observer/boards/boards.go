package boards

import "fmt"

type Post struct {
	PostID string
	Author string
	Title  string
	Body   string
	Tick   uint64
}

type Board struct {
	BoardID string
	Posts   []Post
}

func NewPostID(n uint64) string {
	return fmt.Sprintf("P%06d", n)
}
