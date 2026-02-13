package world

type observerPostingWorldEnv struct {
	w *World
}

func (e observerPostingWorldEnv) ParseContainerID(id string) (typ string, pos Vec3i, ok bool) {
	if e.w == nil {
		return "", Vec3i{}, false
	}
	return parseContainerID(id)
}

func (e observerPostingWorldEnv) CanonicalBoardID(pos Vec3i) string {
	return boardIDAt(pos)
}

func (e observerPostingWorldEnv) BlockNameAt(pos Vec3i) string {
	if e.w == nil {
		return ""
	}
	return e.w.blockName(e.w.chunks.GetBlock(pos))
}

func (e observerPostingWorldEnv) Distance(a Vec3i, b Vec3i) int {
	return Manhattan(a, b)
}

func (e observerPostingWorldEnv) PostingAllowed(agentID string, pos Vec3i) bool {
	if e.w == nil {
		return false
	}
	land := e.w.landAt(pos)
	return land == nil || e.w.isLandMember(agentID, land) || land.Flags.AllowTrade
}

func (e observerPostingWorldEnv) GetBoard(boardID string) *Board {
	if e.w == nil {
		return nil
	}
	return e.w.boards[boardID]
}

func (e observerPostingWorldEnv) EnsureBoard(pos Vec3i) *Board {
	if e.w == nil {
		return nil
	}
	return e.w.ensureBoard(pos)
}

func (e observerPostingWorldEnv) PutBoard(boardID string, board *Board) {
	if e.w == nil || board == nil {
		return
	}
	e.w.boards[boardID] = board
}

func (e observerPostingWorldEnv) NewPostID() string {
	if e.w == nil {
		return ""
	}
	return e.w.newPostID()
}

func (e observerPostingWorldEnv) AuditBoardPost(nowTick uint64, actorID string, pos Vec3i, boardID string, postID string, title string) {
	if e.w == nil {
		return
	}
	e.w.auditEvent(nowTick, actorID, "BOARD_POST", pos, "POST_BOARD", map[string]any{
		"board_id": boardID,
		"post_id":  postID,
		"title":    title,
	})
}

func (e observerPostingWorldEnv) CanBuildAt(agentID string, pos Vec3i, nowTick uint64) bool {
	if e.w == nil {
		return false
	}
	return e.w.canBuildAt(agentID, pos, nowTick)
}

func (e observerPostingWorldEnv) EnsureSign(pos Vec3i) *Sign {
	if e.w == nil {
		return nil
	}
	return e.w.ensureSign(pos)
}

func (e observerPostingWorldEnv) SignIDAt(pos Vec3i) string {
	return signIDAt(pos)
}

func (e observerPostingWorldEnv) AuditSignSet(nowTick uint64, actorID string, pos Vec3i, signID string, text string) {
	if e.w == nil {
		return
	}
	e.w.auditEvent(nowTick, actorID, "SIGN_SET", pos, "SET_SIGN", map[string]any{
		"sign_id":     signID,
		"text":        text,
		"text_length": len(text),
	})
}

func (e observerPostingWorldEnv) BumpLawRep(agentID string, delta int) {
	if e.w == nil {
		return
	}
	e.w.bumpRepLaw(agentID, delta)
}

func (e observerPostingWorldEnv) RecordDenied(nowTick uint64) {
	if e.w == nil || e.w.stats == nil {
		return
	}
	e.w.stats.RecordDenied(nowTick)
}
