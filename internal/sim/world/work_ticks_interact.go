package world

import (
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	interactpkg "voxelcraft.ai/internal/sim/world/feature/work/interact"
)

func (w *World) tickOpen(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	c := w.getContainerByID(wt.TargetID)
	if c == nil {
		// Fallback: allow OPEN on bulletin boards ("BULLETIN_BOARD@x,y,z") to read posts.
		if typ, pos, ok := parseContainerID(wt.TargetID); ok && typ == "BULLETIN_BOARD" {
			if ok, code, msg := interactpkg.ValidateBoardOpen(w.blockName(w.chunks.GetBlock(pos)), Manhattan(a.Pos, pos)); !ok {
				a.WorkTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
				return
			}
			bid := boardIDAt(pos)
			b := w.boards[bid]
			if b == nil {
				b = w.ensureBoard(pos)
			}

			boardPosts := make([]interactpkg.BoardPost, 0, len(b.Posts))
			for _, p := range b.Posts {
				boardPosts = append(boardPosts, interactpkg.BoardPost{
					PostID: p.PostID,
					Author: p.Author,
					Title:  p.Title,
					Body:   p.Body,
					Tick:   p.Tick,
				})
			}
			posts := interactpkg.BuildBoardPosts(boardPosts, 20)
			a.AddEvent(protocol.Event{
				"t":           nowTick,
				"type":        "BOARD",
				"board_id":    bid,
				"pos":         pos.ToArray(),
				"total_posts": len(b.Posts),
				"posts":       posts,
			})
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
			return
		}

		// Fallback: allow OPEN on signs ("SIGN@x,y,z") to read text.
		if typ, pos, ok := parseContainerID(wt.TargetID); ok && typ == "SIGN" {
			if ok, code, msg := interactpkg.ValidateSignOpen(w.blockName(w.chunks.GetBlock(pos)), Manhattan(a.Pos, pos)); !ok {
				a.WorkTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
				return
			}
			s := w.signs[pos]
			text := ""
			updatedTick := uint64(0)
			updatedBy := ""
			if s != nil {
				text = s.Text
				updatedTick = s.UpdatedTick
				updatedBy = s.UpdatedBy
			}
			a.AddEvent(protocol.Event{
				"t":            nowTick,
				"type":         "SIGN",
				"sign_id":      signIDAt(pos),
				"pos":          pos.ToArray(),
				"text":         text,
				"updated_tick": updatedTick,
				"updated_by":   updatedBy,
			})
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
			return
		}

		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_INVALID_TARGET", "message": "container not found"})
		return
	}
	if Manhattan(a.Pos, c.Pos) > 3 {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_BLOCKED", "message": "too far"})
		return
	}

	ev := protocol.Event{
		"t":              nowTick,
		"type":           "CONTAINER",
		"container":      c.ID(),
		"container_type": c.Type,
		"pos":            c.Pos.ToArray(),
		"inventory":      c.InventoryList(),
	}
	// Include owed summary for this agent.
	if c.Owed != nil {
		if owed := c.Owed[a.ID]; owed != nil {
			ev["owed"] = inventorypkg.EncodeItemPairs(owed)
		}
	}
	// Include contract summaries if it's a terminal.
	if c.Type == "CONTRACT_TERMINAL" {
		ev["contracts"] = w.contractSummariesForTerminal(c.Pos)
	}
	a.AddEvent(ev)
	w.onContainerOpenedDuringEvent(a, c, nowTick)

	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}

func (w *World) tickTransfer(a *Agent, wt *tasks.WorkTask, nowTick uint64) {
	srcID := wt.SrcContainer
	dstID := wt.DstContainer
	item := wt.ItemID
	n := wt.Count

	if ok, code, msg := interactpkg.ValidateTransferNoop(srcID, dstID); !ok {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
		return
	}

	var srcC *Container
	var dstC *Container
	if srcID != "SELF" {
		srcC = w.getContainerByID(srcID)
		srcDist := 0
		if srcC != nil {
			srcDist = Manhattan(a.Pos, srcC.Pos)
		}
		if ok, code, msg := interactpkg.ValidateContainerDistance(srcC != nil, srcDist, "src"); !ok {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
			return
		}
	}
	if dstID != "SELF" {
		dstC = w.getContainerByID(dstID)
		dstDist := 0
		if dstC != nil {
			dstDist = Manhattan(a.Pos, dstC.Pos)
		}
		if ok, code, msg := interactpkg.ValidateContainerDistance(dstC != nil, dstDist, "dst"); !ok {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
			return
		}
	}

	// Withdraw permission and escrow protection.
	if srcC != nil {
		if !w.canWithdrawFromContainer(a.ID, srcC.Pos) {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "withdraw denied"})
			return
		}
		if srcC.availableCount(item) < n {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "insufficient src items"})
			return
		}
	} else {
		if a.Inventory[item] < n {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_RESOURCE", "message": "insufficient self items"})
			return
		}
	}

	// Execute transfer.
	if srcC != nil {
		srcC.Inventory[item] -= n
		if srcC.Inventory[item] <= 0 {
			delete(srcC.Inventory, item)
		}
	} else {
		a.Inventory[item] -= n
	}
	if dstC != nil {
		if dstC.Inventory == nil {
			dstC.Inventory = map[string]int{}
		}
		dstC.Inventory[item] += n
	} else {
		a.Inventory[item] += n
	}

	// Audit the transfer for dispute resolution.
	ap := a.Pos
	if dstC != nil {
		ap = dstC.Pos
	} else if srcC != nil {
		ap = srcC.Pos
	}
	w.auditEvent(nowTick, a.ID, "TRANSFER", ap, "TRANSFER", map[string]any{
		"src":   srcID,
		"dst":   dstID,
		"item":  item,
		"count": n,
	})

	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}
