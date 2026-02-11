package world

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
	featureobserver "voxelcraft.ai/internal/sim/world/feature/observer"
)

func handleInstantPostBoard(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, cd := a.RateLimitAllow("POST_BOARD", nowTick, uint64(w.cfg.RateLimits.PostBoardWindowTicks), w.cfg.RateLimits.PostBoardMax); !ok {
		ev := actionResult(nowTick, inst.ID, false, "E_RATE_LIMIT", "too many POST_BOARD")
		ev["cooldown_ticks"] = cd
		ev["cooldown_until_tick"] = nowTick + cd
		a.AddEvent(ev)
		return
	}
	boardID := featureobserver.ResolveBoardID(inst.BoardID, inst.TargetID)
	if ok, code, message := featureobserver.ValidatePostInput(boardID, inst.Title, inst.Body); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, message))
		return
	}

	// Physical bulletin boards are addressed by id "BULLETIN_BOARD@x,y,z" and require proximity.
	physical := false
	var postPos Vec3i
	if typ, pos, ok := parseContainerID(boardID); ok {
		if typ != "BULLETIN_BOARD" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "invalid board target"))
			return
		}
		if w.blockName(w.chunks.GetBlock(pos)) != "BULLETIN_BOARD" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "bulletin board not found"))
			return
		}
		if Manhattan(a.Pos, pos) > 3 {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
			return
		}
		// Posting in claimed land may be restricted by allow_trade for visitors.
		if land := w.landAt(pos); land != nil && !w.isLandMember(a.ID, land) && !land.Flags.AllowTrade {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "posting not allowed here"))
			return
		}
		physical = true
		postPos = pos
		boardID = boardIDAt(pos) // canonicalize
	}

	b := w.boards[boardID]
	if b == nil {
		if physical {
			b = w.ensureBoard(postPos)
		} else {
			b = &Board{BoardID: boardID}
			w.boards[boardID] = b
		}
	}
	postID := w.newPostID()
	b.Posts = append(b.Posts, BoardPost{
		PostID: postID,
		Author: a.ID,
		Title:  inst.Title,
		Body:   inst.Body,
		Tick:   nowTick,
	})
	w.auditEvent(nowTick, a.ID, "BOARD_POST", postPos, "POST_BOARD", map[string]any{
		"board_id": boardID,
		"post_id":  postID,
		"title":    inst.Title,
	})
	a.AddEvent(protocol.Event{"t": nowTick, "type": "ACTION_RESULT", "ref": inst.ID, "ok": true, "post_id": postID})
}

func handleInstantSearchBoard(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	boardID := featureobserver.ResolveBoardID(inst.BoardID, inst.TargetID)
	query := strings.TrimSpace(inst.Text)
	if ok, code, message := featureobserver.ValidateSearchInput(boardID, query); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, message))
		return
	}

	limit := featureobserver.NormalizeBoardSearchLimit(inst.Limit)

	// Physical bulletin boards are addressed by id "BULLETIN_BOARD@x,y,z" and require proximity.
	if typ, pos, ok := parseContainerID(boardID); ok && typ == "BULLETIN_BOARD" {
		if w.blockName(w.chunks.GetBlock(pos)) != "BULLETIN_BOARD" {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "bulletin board not found"))
			return
		}
		if Manhattan(a.Pos, pos) > 3 {
			a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
			return
		}
		boardID = boardIDAt(pos) // canonicalize
		if w.boards[boardID] == nil {
			w.ensureBoard(pos)
		}
	}

	b := w.boards[boardID]
	if b == nil {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "board not found"))
		return
	}

	results := featureobserver.MatchBoardPosts(b.Posts, query, limit)
	a.AddEvent(protocol.Event{
		"t":           nowTick,
		"type":        "BOARD_SEARCH",
		"board_id":    boardID,
		"query":       query,
		"total_posts": len(b.Posts),
		"results":     results,
	})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}

func handleInstantSetSign(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	target := strings.TrimSpace(inst.TargetID)
	if target == "" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "missing target_id"))
		return
	}
	typ, pos, ok := parseContainerID(target)
	if !ok || typ != "SIGN" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "invalid sign target"))
		return
	}
	if w.blockName(w.chunks.GetBlock(pos)) != "SIGN" {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_INVALID_TARGET", "sign not found"))
		return
	}
	if Manhattan(a.Pos, pos) > 3 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BLOCKED", "too far"))
		return
	}
	if len(inst.Text) > 200 {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "text too large"))
		return
	}
	if !w.canBuildAt(a.ID, pos, nowTick) {
		w.bumpRepLaw(a.ID, -1)
		if w.stats != nil {
			w.stats.RecordDenied(nowTick)
		}
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_NO_PERMISSION", "sign edit denied"))
		return
	}

	s := w.ensureSign(pos)
	s.Text = inst.Text
	s.UpdatedTick = nowTick
	s.UpdatedBy = a.ID
	w.auditEvent(nowTick, a.ID, "SIGN_SET", pos, "SET_SIGN", map[string]any{
		"sign_id":     signIDAt(pos),
		"text":        inst.Text,
		"text_length": len(inst.Text),
	})
	a.AddEvent(actionResult(nowTick, inst.ID, true, "", "ok"))
}
