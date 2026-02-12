package world

import (
	"strings"

	"voxelcraft.ai/internal/protocol"
	postingpkg "voxelcraft.ai/internal/sim/world/feature/observer/posting"
	searchpkg "voxelcraft.ai/internal/sim/world/feature/observer/search"
	targetspkg "voxelcraft.ai/internal/sim/world/feature/observer/targets"
)

func handleInstantPostBoard(w *World, a *Agent, inst protocol.InstantReq, nowTick uint64) {
	if ok, cd := a.RateLimitAllow("POST_BOARD", nowTick, uint64(w.cfg.RateLimits.PostBoardWindowTicks), w.cfg.RateLimits.PostBoardMax); !ok {
		ev := actionResult(nowTick, inst.ID, false, "E_RATE_LIMIT", "too many POST_BOARD")
		ev["cooldown_ticks"] = cd
		ev["cooldown_until_tick"] = nowTick + cd
		a.AddEvent(ev)
		return
	}
	boardID := postingpkg.ResolveBoardID(inst.BoardID, inst.TargetID)
	if ok, code, message := postingpkg.ValidatePostInput(boardID, inst.Title, inst.Body); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, message))
		return
	}

	// Physical bulletin boards are addressed by id "BULLETIN_BOARD@x,y,z" and require proximity.
	physical := false
	var postPos Vec3i
	if typ, pos, ok := parseContainerID(boardID); ok {
		// Posting in claimed land may be restricted by allow_trade for visitors.
		postingAllowed := true
		if land := w.landAt(pos); land != nil && !w.isLandMember(a.ID, land) && !land.Flags.AllowTrade {
			postingAllowed = false
		}
		if ok, code, message := targetspkg.ValidatePhysicalBoardTarget(
			typ,
			w.blockName(w.chunks.GetBlock(pos)),
			Manhattan(a.Pos, pos),
			postingAllowed,
		); !ok {
			a.AddEvent(actionResult(nowTick, inst.ID, false, code, message))
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
	boardID := postingpkg.ResolveBoardID(inst.BoardID, inst.TargetID)
	query := strings.TrimSpace(inst.Text)
	if ok, code, message := postingpkg.ValidateSearchInput(boardID, query); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, message))
		return
	}

	limit := searchpkg.NormalizeBoardSearchLimit(inst.Limit)

	// Physical bulletin boards are addressed by id "BULLETIN_BOARD@x,y,z" and require proximity.
	if typ, pos, ok := parseContainerID(boardID); ok && typ == "BULLETIN_BOARD" {
		if ok, code, message := targetspkg.ValidatePhysicalBoardTarget(
			typ,
			w.blockName(w.chunks.GetBlock(pos)),
			Manhattan(a.Pos, pos),
			true,
		); !ok {
			a.AddEvent(actionResult(nowTick, inst.ID, false, code, message))
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

	results := searchpkg.MatchBoardPosts(b.Posts, query, limit)
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
	if !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, "E_BAD_REQUEST", "invalid sign target"))
		return
	}
	if ok, code, message := targetspkg.ValidateSetSignTarget(
		typ,
		w.blockName(w.chunks.GetBlock(pos)),
		Manhattan(a.Pos, pos),
		len(inst.Text),
	); !ok {
		a.AddEvent(actionResult(nowTick, inst.ID, false, code, message))
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
