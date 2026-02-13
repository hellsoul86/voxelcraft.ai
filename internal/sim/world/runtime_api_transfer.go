package world

import (
	"context"
	"errors"

	"voxelcraft.ai/internal/protocol"
	transfereventspkg "voxelcraft.ai/internal/sim/world/feature/transfer/events"
	transferruntimepkg "voxelcraft.ai/internal/sim/world/feature/transfer/runtime"
)

type EventCursorItem = transfereventspkg.CursorItem

type injectEventReq struct {
	AgentID string
	Event   protocol.Event
}

func (w *World) RequestEventsAfter(ctx context.Context, agentID string, sinceCursor uint64, limit int) ([]EventCursorItem, uint64, error) {
	if w == nil || w.eventsReq == nil {
		return nil, sinceCursor, errors.New("event query not available")
	}
	req := transfereventspkg.Req{
		AgentID:     agentID,
		SinceCursor: sinceCursor,
		Limit:       limit,
		Resp:        make(chan transfereventspkg.Resp, 1),
	}
	select {
	case w.eventsReq <- req:
	case <-ctx.Done():
		return nil, sinceCursor, ctx.Err()
	}
	select {
	case resp := <-req.Resp:
		if resp.Err != "" {
			return nil, sinceCursor, errors.New(resp.Err)
		}
		return resp.Items, resp.NextCursor, nil
	case <-ctx.Done():
		return nil, sinceCursor, ctx.Err()
	}
}

func (w *World) RequestInjectEvent(ctx context.Context, agentID string, ev protocol.Event) error {
	if w == nil || w.injectEvent == nil {
		return errors.New("inject event not available")
	}
	req := injectEventReq{AgentID: agentID, Event: ev}
	select {
	case w.injectEvent <- req:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

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

func (w *World) handleEventsReq(req transfereventspkg.Req) {
	resp := transferruntimepkg.HandleEventsReq(req, func(agentID string, sinceCursor uint64, limit int) ([]transfereventspkg.CursorItem, uint64, bool) {
		a := w.agents[agentID]
		if a == nil {
			return nil, sinceCursor, false
		}
		items, next := a.EventsAfter(sinceCursor, limit)
		out := make([]transfereventspkg.CursorItem, 0, len(items))
		for _, it := range items {
			out = append(out, transfereventspkg.CursorItem{Cursor: it.Cursor, Event: it.Event})
		}
		return out, next, true
	})
	if req.Resp == nil {
		return
	}
	select {
	case req.Resp <- resp:
	default:
	}
}

// RequestAgentPos returns the current position for an agent from the world loop goroutine.
func (w *World) RequestAgentPos(ctx context.Context, agentID string) (Vec3i, error) {
	pos, err := transferruntimepkg.RequestAgentPos(ctx, w.agentPosReq, agentID)
	if err != nil {
		return Vec3i{}, err
	}
	return Vec3i{X: pos[0], Y: pos[1], Z: pos[2]}, nil
}

func (w *World) handleAgentPosReq(req transferruntimepkg.AgentPosReq) {
	resp := transferruntimepkg.HandleAgentPosReq(req, func(agentID string) ([3]int, bool) {
		a := w.agents[agentID]
		if a == nil {
			return [3]int{}, false
		}
		return a.Pos.ToArray(), true
	})
	if req.Resp == nil {
		return
	}
	select {
	case req.Resp <- resp:
	default:
	}
}

// RequestOrgMetaSnapshot returns a world-local snapshot of organization metadata.
// Treasury is intentionally excluded; this API is used for cross-world identity sync.
func (w *World) RequestOrgMetaSnapshot(ctx context.Context) ([]OrgTransfer, error) {
	states, err := transferruntimepkg.RequestOrgMetaSnapshot(ctx, w.orgMetaReq)
	if err != nil {
		return nil, err
	}
	return transferruntimepkg.TransfersFromStates(states), nil
}

func (w *World) handleOrgMetaReq(req transferruntimepkg.OrgMetaReq) {
	resp := transferruntimepkg.OrgMetaResp{}
	if w == nil {
		resp.Err = "world unavailable"
		if req.Resp == nil {
			return
		}
		select {
		case req.Resp <- resp:
		default:
		}
		return
	}
	resp = transferruntimepkg.SnapshotOrgMeta(w.orgs)
	if req.Resp == nil {
		return
	}
	select {
	case req.Resp <- resp:
	default:
	}
}

// RequestUpsertOrgMeta applies manager-authoritative org metadata into this world.
// It updates identity/membership only; treasury remains world-local.
func (w *World) RequestUpsertOrgMeta(ctx context.Context, orgs []OrgTransfer) error {
	if w == nil || w.orgMetaUpsert == nil {
		return errors.New("org metadata upsert not available")
	}
	incoming := transferruntimepkg.StatesFromTransfers(orgs)
	return transferruntimepkg.RequestOrgMetaUpsert(ctx, w.orgMetaUpsert, incoming)
}

func (w *World) handleOrgMetaUpsertReq(req transferruntimepkg.OrgMetaUpsertReq) {
	resp := transferruntimepkg.OrgMetaUpsertResp{}
	defer func() {
		if req.Resp == nil {
			return
		}
		select {
		case req.Resp <- resp:
		default:
		}
	}()
	if w == nil {
		resp.Err = "world unavailable"
		return
	}

	transferruntimepkg.ApplyOrgMetaUpsert(transferruntimepkg.ApplyOrgMetaUpsertInput{
		Orgs:     w.orgs,
		Agents:   w.agents,
		Incoming: req.Orgs,
		OnOrg: func(org *Organization) {
			_ = w.orgTreasury(org)
		},
	})
}
