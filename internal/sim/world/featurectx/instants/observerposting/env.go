package observerposting

import modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"

type Env struct {
	ParseContainerIDFn func(id string) (typ string, pos modelpkg.Vec3i, ok bool)
	CanonicalBoardIDFn func(pos modelpkg.Vec3i) string
	BlockNameAtFn      func(pos modelpkg.Vec3i) string
	DistanceFn         func(a modelpkg.Vec3i, b modelpkg.Vec3i) int
	PostingAllowedFn   func(agentID string, pos modelpkg.Vec3i) bool
	GetBoardFn         func(boardID string) *modelpkg.Board
	EnsureBoardFn      func(pos modelpkg.Vec3i) *modelpkg.Board
	PutBoardFn         func(boardID string, board *modelpkg.Board)
	NewPostIDFn        func() string
	AuditBoardPostFn   func(nowTick uint64, actorID string, pos modelpkg.Vec3i, boardID string, postID string, title string)
	CanBuildAtFn       func(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool
	EnsureSignFn       func(pos modelpkg.Vec3i) *modelpkg.Sign
	SignIDAtFn         func(pos modelpkg.Vec3i) string
	AuditSignSetFn     func(nowTick uint64, actorID string, pos modelpkg.Vec3i, signID string, text string)
	BumpLawRepFn       func(agentID string, delta int)
	RecordDeniedFn     func(nowTick uint64)
}

func (e Env) ParseContainerID(id string) (typ string, pos modelpkg.Vec3i, ok bool) {
	if e.ParseContainerIDFn == nil {
		return "", modelpkg.Vec3i{}, false
	}
	return e.ParseContainerIDFn(id)
}

func (e Env) CanonicalBoardID(pos modelpkg.Vec3i) string {
	if e.CanonicalBoardIDFn == nil {
		return ""
	}
	return e.CanonicalBoardIDFn(pos)
}

func (e Env) BlockNameAt(pos modelpkg.Vec3i) string {
	if e.BlockNameAtFn == nil {
		return ""
	}
	return e.BlockNameAtFn(pos)
}

func (e Env) Distance(a modelpkg.Vec3i, b modelpkg.Vec3i) int {
	if e.DistanceFn == nil {
		return 0
	}
	return e.DistanceFn(a, b)
}

func (e Env) PostingAllowed(agentID string, pos modelpkg.Vec3i) bool {
	if e.PostingAllowedFn == nil {
		return false
	}
	return e.PostingAllowedFn(agentID, pos)
}

func (e Env) GetBoard(boardID string) *modelpkg.Board {
	if e.GetBoardFn == nil {
		return nil
	}
	return e.GetBoardFn(boardID)
}

func (e Env) EnsureBoard(pos modelpkg.Vec3i) *modelpkg.Board {
	if e.EnsureBoardFn == nil {
		return nil
	}
	return e.EnsureBoardFn(pos)
}

func (e Env) PutBoard(boardID string, board *modelpkg.Board) {
	if e.PutBoardFn != nil {
		e.PutBoardFn(boardID, board)
	}
}

func (e Env) NewPostID() string {
	if e.NewPostIDFn == nil {
		return ""
	}
	return e.NewPostIDFn()
}

func (e Env) AuditBoardPost(nowTick uint64, actorID string, pos modelpkg.Vec3i, boardID string, postID string, title string) {
	if e.AuditBoardPostFn != nil {
		e.AuditBoardPostFn(nowTick, actorID, pos, boardID, postID, title)
	}
}

func (e Env) CanBuildAt(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool {
	if e.CanBuildAtFn == nil {
		return false
	}
	return e.CanBuildAtFn(agentID, pos, nowTick)
}

func (e Env) EnsureSign(pos modelpkg.Vec3i) *modelpkg.Sign {
	if e.EnsureSignFn == nil {
		return nil
	}
	return e.EnsureSignFn(pos)
}

func (e Env) SignIDAt(pos modelpkg.Vec3i) string {
	if e.SignIDAtFn == nil {
		return ""
	}
	return e.SignIDAtFn(pos)
}

func (e Env) AuditSignSet(nowTick uint64, actorID string, pos modelpkg.Vec3i, signID string, text string) {
	if e.AuditSignSetFn != nil {
		e.AuditSignSetFn(nowTick, actorID, pos, signID, text)
	}
}

func (e Env) BumpLawRep(agentID string, delta int) {
	if e.BumpLawRepFn != nil {
		e.BumpLawRepFn(agentID, delta)
	}
}

func (e Env) RecordDenied(nowTick uint64) {
	if e.RecordDeniedFn != nil {
		e.RecordDeniedFn(nowTick)
	}
}
