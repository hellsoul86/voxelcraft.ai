package world

import transferruntimepkg "voxelcraft.ai/internal/sim/world/feature/transfer/runtime"

func (w *World) handleTransferOut(req transferOutReq) {
	resp := transferOutResp{}
	defer func() {
		if req.Resp == nil {
			return
		}
		select {
		case req.Resp <- resp:
		default:
		}
	}()

	a := w.agents[req.AgentID]
	if a == nil {
		resp.Err = "agent not found"
		return
	}

	// Cancel tasks on world switch.
	a.MoveTask = nil
	a.WorkTask = nil

	var orgTransfer *OrgTransfer
	if a.OrgID != "" {
		if org := w.orgByID(a.OrgID); org != nil {
			orgTransfer = transferruntimepkg.BuildOrgTransferFromOrganization(org)
		}
	}
	resp.Transfer = transferruntimepkg.BuildOutgoingAgent(a, w.cfg.ID, orgTransfer)

	delete(w.clients, a.ID)
	delete(w.agents, a.ID)

	// Clear open trades involving this agent in this world.
	for tid, tr := range w.trades {
		if tr == nil {
			continue
		}
		if tr.From == a.ID || tr.To == a.ID {
			delete(w.trades, tid)
		}
	}
}
