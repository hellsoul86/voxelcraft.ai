package observer

import "fmt"

type BoardPost struct {
	PostID string
	Author string
	Title  string
	Body   string
	Tick   uint64
}

type Board struct {
	BoardID string
	Posts   []BoardPost // append-only, newest last
}

func NewPostID(n uint64) string {
	return fmt.Sprintf("P%06d", n)
}
