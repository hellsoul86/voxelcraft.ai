package world

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

func (w *World) newPostID() string {
	n := w.nextPostNum.Add(1)
	return fmt.Sprintf("P%06d", n)
}
