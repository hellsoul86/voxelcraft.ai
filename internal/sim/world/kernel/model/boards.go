package model

type BoardPost struct {
	PostID string
	Author string
	Title  string
	Body   string
	Tick   uint64
}

type Board struct {
	BoardID string
	Posts   []BoardPost
}

