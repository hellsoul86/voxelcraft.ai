package world

import (
	"context"
	"errors"

	transferruntimepkg "voxelcraft.ai/internal/sim/world/feature/transfer/runtime"
)

func (w *World) RequestTransferOut(ctx context.Context, agentID string) (AgentTransfer, error) {
	if w == nil {
		return AgentTransfer{}, errors.New("transfer out not available")
	}
	return transferruntimepkg.RequestTransferOut(ctx, w.transferOut, agentID)
}

func (w *World) RequestTransferIn(ctx context.Context, t AgentTransfer, out chan []byte, delta bool) error {
	if w == nil {
		return errors.New("transfer in not available")
	}
	return transferruntimepkg.RequestTransferIn(ctx, w.transferIn, t, out, delta)
}

func (w *World) handleTransferIn(req transferruntimepkg.TransferInReq) {
	resp := transferruntimepkg.TransferInResp{}
	defer func() {
		if req.Resp == nil {
			return
		}
		select {
		case req.Resp <- resp:
		default:
		}
	}()

	out := transferruntimepkg.HandleTransferIn(transferruntimepkg.TransferInHandleInput{
		Req:            req,
		WorldID:        w.cfg.ID,
		NowTick:        w.tick.Load(),
		Agents:         w.agents,
		Orgs:           w.orgs,
		CurrentNextOrg: w.nextOrgNum.Load(),
	})
	if out.Err != "" {
		resp.Err = out.Err
		return
	}
	if out.NextOrg > w.nextOrgNum.Load() {
		w.nextOrgNum.Store(out.NextOrg)
	}
	if out.JoinedAgentID == "" {
		resp.Err = "transfer in did not produce agent"
		return
	}
	if req.Out != nil {
		w.clients[out.JoinedAgentID] = &clientState{Out: req.Out, DeltaVoxels: req.DeltaVoxels}
	}
}

func (w *World) handleTransferOut(req transferruntimepkg.TransferOutReq) {
	resp := transferruntimepkg.TransferOutResp{}
	defer func() {
		if req.Resp == nil {
			return
		}
		select {
		case req.Resp <- resp:
		default:
		}
	}()

	out := transferruntimepkg.HandleTransferOut(transferruntimepkg.TransferOutHandleInput{
		Req:     req,
		WorldID: w.cfg.ID,
		Agents:  w.agents,
		Orgs:    w.orgs,
		Trades:  w.trades,
	})
	if out.Err != "" {
		resp.Err = out.Err
		return
	}
	resp.Transfer = out.Transfer
	if out.RemovedID != "" {
		delete(w.clients, out.RemovedID)
	}
}
