package runtime

import (
	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/tasks"
	inventorypkg "voxelcraft.ai/internal/sim/world/feature/economy/inventory"
	interactpkg "voxelcraft.ai/internal/sim/world/feature/work/interact"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type WorkExecInteractEnv interface {
	GetContainerByID(id string) *modelpkg.Container
	ParseContainerID(id string) (typ string, pos modelpkg.Vec3i, ok bool)
	BlockNameAt(pos modelpkg.Vec3i) string
	EnsureBoard(pos modelpkg.Vec3i) *modelpkg.Board
	BoardIDAt(pos modelpkg.Vec3i) string
	GetSign(pos modelpkg.Vec3i) *modelpkg.Sign
	SignIDAt(pos modelpkg.Vec3i) string
	ContractSummariesForTerminal(pos modelpkg.Vec3i) []map[string]interface{}
	OnContainerOpenedDuringEvent(a *modelpkg.Agent, c *modelpkg.Container, nowTick uint64)
	CanWithdrawFromContainer(agentID string, pos modelpkg.Vec3i) bool
	AuditTransfer(nowTick uint64, actorID string, at modelpkg.Vec3i, srcID string, dstID string, item string, count int)
}

func TickOpen(env WorkExecInteractEnv, a *modelpkg.Agent, wt *tasks.WorkTask, nowTick uint64) {
	c := env.GetContainerByID(wt.TargetID)
	if c == nil {
		if typ, pos, ok := env.ParseContainerID(wt.TargetID); ok && typ == "BULLETIN_BOARD" {
			if ok, code, msg := interactpkg.ValidateBoardOpen(env.BlockNameAt(pos), modelpkg.Manhattan(a.Pos, pos)); !ok {
				a.WorkTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
				return
			}
			b := env.EnsureBoard(pos)
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
				"board_id":    env.BoardIDAt(pos),
				"pos":         pos.ToArray(),
				"total_posts": len(b.Posts),
				"posts":       posts,
			})
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
			return
		}

		if typ, pos, ok := env.ParseContainerID(wt.TargetID); ok && typ == "SIGN" {
			if ok, code, msg := interactpkg.ValidateSignOpen(env.BlockNameAt(pos), modelpkg.Manhattan(a.Pos, pos)); !ok {
				a.WorkTask = nil
				a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
				return
			}
			s := env.GetSign(pos)
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
				"sign_id":      env.SignIDAt(pos),
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
	if modelpkg.Manhattan(a.Pos, c.Pos) > 3 {
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
	if c.Owed != nil {
		if owed := c.Owed[a.ID]; owed != nil {
			ev["owed"] = inventorypkg.EncodeItemPairs(owed)
		}
	}
	if c.Type == "CONTRACT_TERMINAL" {
		ev["contracts"] = env.ContractSummariesForTerminal(c.Pos)
	}
	a.AddEvent(ev)
	env.OnContainerOpenedDuringEvent(a, c, nowTick)

	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}

func TickTransfer(env WorkExecInteractEnv, a *modelpkg.Agent, wt *tasks.WorkTask, nowTick uint64) {
	srcID := wt.SrcContainer
	dstID := wt.DstContainer
	item := wt.ItemID
	n := wt.Count

	if ok, code, msg := interactpkg.ValidateTransferNoop(srcID, dstID); !ok {
		a.WorkTask = nil
		a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
		return
	}

	var srcC *modelpkg.Container
	var dstC *modelpkg.Container
	if srcID != "SELF" {
		srcC = env.GetContainerByID(srcID)
		srcDist := 0
		if srcC != nil {
			srcDist = modelpkg.Manhattan(a.Pos, srcC.Pos)
		}
		if ok, code, msg := interactpkg.ValidateContainerDistance(srcC != nil, srcDist, "src"); !ok {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
			return
		}
	}
	if dstID != "SELF" {
		dstC = env.GetContainerByID(dstID)
		dstDist := 0
		if dstC != nil {
			dstDist = modelpkg.Manhattan(a.Pos, dstC.Pos)
		}
		if ok, code, msg := interactpkg.ValidateContainerDistance(dstC != nil, dstDist, "dst"); !ok {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": code, "message": msg})
			return
		}
	}

	if srcC != nil {
		if !env.CanWithdrawFromContainer(a.ID, srcC.Pos) {
			a.WorkTask = nil
			a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_FAIL", "task_id": wt.TaskID, "code": "E_NO_PERMISSION", "message": "withdraw denied"})
			return
		}
		if srcC.AvailableCount(item) < n {
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

	ap := a.Pos
	if dstC != nil {
		ap = dstC.Pos
	} else if srcC != nil {
		ap = srcC.Pos
	}
	env.AuditTransfer(nowTick, a.ID, ap, srcID, dstID, item, n)

	a.WorkTask = nil
	a.AddEvent(protocol.Event{"t": nowTick, "type": "TASK_DONE", "task_id": wt.TaskID, "kind": string(wt.Kind)})
}
