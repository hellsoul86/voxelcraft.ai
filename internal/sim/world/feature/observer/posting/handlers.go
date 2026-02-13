package posting

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
	searchpkg "voxelcraft.ai/internal/sim/world/feature/observer/search"
	targetspkg "voxelcraft.ai/internal/sim/world/feature/observer/targets"
)

type ActionResultFn func(tick uint64, ref string, ok bool, code string, message string) protocol.Event

type PostingRateLimits struct {
	PostWindowTicks int
	PostMax         int
}

type PostingEnv interface {
	ParseContainerID(id string) (typ string, pos modelpkg.Vec3i, ok bool)
	CanonicalBoardID(pos modelpkg.Vec3i) string
	BlockNameAt(pos modelpkg.Vec3i) string
	Distance(a modelpkg.Vec3i, b modelpkg.Vec3i) int
	PostingAllowed(agentID string, pos modelpkg.Vec3i) bool
	GetBoard(boardID string) *modelpkg.Board
	EnsureBoard(pos modelpkg.Vec3i) *modelpkg.Board
	PutBoard(boardID string, board *modelpkg.Board)
	NewPostID() string
	AuditBoardPost(nowTick uint64, actorID string, pos modelpkg.Vec3i, boardID string, postID string, title string)
}

type SignEnv interface {
	ParseContainerID(id string) (typ string, pos modelpkg.Vec3i, ok bool)
	BlockNameAt(pos modelpkg.Vec3i) string
	Distance(a modelpkg.Vec3i, b modelpkg.Vec3i) int
	CanBuildAt(agentID string, pos modelpkg.Vec3i, nowTick uint64) bool
	EnsureSign(pos modelpkg.Vec3i) *modelpkg.Sign
	SignIDAt(pos modelpkg.Vec3i) string
	AuditSignSet(nowTick uint64, actorID string, pos modelpkg.Vec3i, signID string, text string)
	BumpLawRep(agentID string, delta int)
	RecordDenied(nowTick uint64)
}

func HandlePostBoard(env PostingEnv, ar ActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64, limits PostingRateLimits) {
	if ok, cd := a.RateLimitAllow("POST_BOARD", nowTick, uint64(limits.PostWindowTicks), limits.PostMax); !ok {
		ev := ar(nowTick, inst.ID, false, "E_RATE_LIMIT", "too many POST_BOARD")
		ev["cooldown_ticks"] = cd
		ev["cooldown_until_tick"] = nowTick + cd
		a.AddEvent(ev)
		return
	}
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "posting env unavailable"))
		return
	}
	boardID := ResolveBoardID(inst.BoardID, inst.TargetID)
	if ok, code, message := ValidatePostInput(boardID, inst.Title, inst.Body); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, message))
		return
	}

	physical := false
	postPos := modelpkg.Vec3i{}
	if typ, pos, ok := env.ParseContainerID(boardID); ok {
		if ok, code, message := targetspkg.ValidatePhysicalBoardTarget(
			typ,
			env.BlockNameAt(pos),
			env.Distance(a.Pos, pos),
			env.PostingAllowed(a.ID, pos),
		); !ok {
			a.AddEvent(ar(nowTick, inst.ID, false, code, message))
			return
		}
		physical = true
		postPos = pos
		boardID = env.CanonicalBoardID(pos)
	}

	b := env.GetBoard(boardID)
	if b == nil {
		if physical {
			b = env.EnsureBoard(postPos)
		} else {
			b = &modelpkg.Board{BoardID: boardID}
			env.PutBoard(boardID, b)
		}
	}
	postID := env.NewPostID()
	b.Posts = append(b.Posts, modelpkg.BoardPost{
		PostID: postID,
		Author: a.ID,
		Title:  inst.Title,
		Body:   inst.Body,
		Tick:   nowTick,
	})
	env.AuditBoardPost(nowTick, a.ID, postPos, boardID, postID, inst.Title)
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "post_id": postID})
}

func HandleSearchBoard(env PostingEnv, ar ActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "posting env unavailable"))
		return
	}
	boardID := ResolveBoardID(inst.BoardID, inst.TargetID)
	query := strings.TrimSpace(inst.Text)
	if ok, code, message := ValidateSearchInput(boardID, query); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, message))
		return
	}

	limit := searchpkg.NormalizeBoardSearchLimit(inst.Limit)
	if typ, pos, ok := env.ParseContainerID(boardID); ok && typ == "BULLETIN_BOARD" {
		if ok, code, message := targetspkg.ValidatePhysicalBoardTarget(
			typ,
			env.BlockNameAt(pos),
			env.Distance(a.Pos, pos),
			true,
		); !ok {
			a.AddEvent(ar(nowTick, inst.ID, false, code, message))
			return
		}
		boardID = env.CanonicalBoardID(pos)
		if env.GetBoard(boardID) == nil {
			env.EnsureBoard(pos)
		}
	}

	b := env.GetBoard(boardID)
	if b == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INVALID_TARGET", "board not found"))
		return
	}

	results := searchpkg.MatchBoardPosts(b.Posts, query, limit)
	a.AddEvent(protocol.Event{
		"t":           nowTick,
		"type":        "BOARD_SEARCH",
		"board_id":    boardID,
		"query":       query,
		"total_posts": len(b.Posts),
		"results":     results,
	})
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}

func HandleSetSign(env SignEnv, ar ActionResultFn, a *modelpkg.Agent, inst protocol.InstantReq, nowTick uint64) {
	if env == nil {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_INTERNAL", "posting env unavailable"))
		return
	}
	target := strings.TrimSpace(inst.TargetID)
	if target == "" {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	typ, pos, ok := env.ParseContainerID(target)
	if !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, "E_BAD_REQUEST", "invalid sign target"))
		return
	}
	if ok, code, message := targetspkg.ValidateSetSignTarget(
		typ,
		env.BlockNameAt(pos),
		env.Distance(a.Pos, pos),
		len(inst.Text),
	); !ok {
		a.AddEvent(ar(nowTick, inst.ID, false, code, message))
		return
	}
	if !env.CanBuildAt(a.ID, pos, nowTick) {
		env.BumpLawRep(a.ID, -1)
		env.RecordDenied(nowTick)
		a.AddEvent(ar(nowTick, inst.ID, false, "E_NO_PERMISSION", "sign edit denied"))
		return
	}

	s := env.EnsureSign(pos)
	s.Text = inst.Text
	s.UpdatedTick = nowTick
	s.UpdatedBy = a.ID
	env.AuditSignSet(nowTick, a.ID, pos, env.SignIDAt(pos), inst.Text)
	a.AddEvent(ar(nowTick, inst.ID, true, "", "ok"))
}
