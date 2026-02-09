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

func boardIDAt(pos Vec3i) string {
	return containerID("BULLETIN_BOARD", pos)
}

func (w *World) ensureBoard(pos Vec3i) *Board {
	id := boardIDAt(pos)
	b := w.boards[id]
	if b != nil {
		return b
	}
	b = &Board{BoardID: id}
	w.boards[id] = b
	return b
}

func (w *World) removeBoard(pos Vec3i) {
	delete(w.boards, boardIDAt(pos))
}
