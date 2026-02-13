package world

import (
	"context"
	"errors"

	"voxelcraft.ai/internal/protocol"
	transferagentpkg "voxelcraft.ai/internal/sim/world/feature/transfer/agent"
	transfereventspkg "voxelcraft.ai/internal/sim/world/feature/transfer/events"
	transferorgpkg "voxelcraft.ai/internal/sim/world/feature/transfer/org"
	transferruntimepkg "voxelcraft.ai/internal/sim/world/feature/transfer/runtime"
	idspkg "voxelcraft.ai/internal/sim/world/logic/ids"
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

	t := req.Transfer
	if t.ID == "" {
		resp.Err = "missing agent id"
		return
	}
	if _, ok := w.agents[t.ID]; ok {
		resp.Err = "agent already present"
		return
	}

	a := transferruntimepkg.BuildIncomingAgent(t, w.cfg.ID)
	if a.OrgID != "" {
		orgID := a.OrgID
		if t.Org != nil && t.Org.OrgID != "" {
			orgID = t.Org.OrgID
		}
		_, existed := w.orgs[orgID]
		org := transferruntimepkg.UpsertIncomingOrg(w.orgs, t.Org, a.OrgID, a.ID)
		if org != nil {
			if !existed {
				if n, ok := idspkg.ParseUintAfterPrefix("ORG", org.OrgID); ok && n > w.nextOrgNum.Load() {
					w.nextOrgNum.Store(n)
				}
			}
			_ = w.orgTreasury(org)
		}
	}
	if ev, ok := transferagentpkg.BuildWorldSwitchEvent(w.tick.Load(), t.FromWorldID, w.cfg.ID, a.ID, t.FromEntryPointID, t.ToEntryPointID); ok {
		a.AddEvent(ev)
	}

	w.agents[a.ID] = a
	if req.Out != nil {
		w.clients[a.ID] = &clientState{Out: req.Out, DeltaVoxels: req.DeltaVoxels}
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
	states := transferorgpkg.StatesFromOrganizations(w.orgs)
	resp = transferruntimepkg.BuildOrgMetaResp(states)
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

	existingStates := transferorgpkg.StatesFromOrganizations(w.orgs)
	mergedStates, ownerByAgent := transferruntimepkg.BuildOrgMetaMerge(existingStates, req.Orgs)
	transferorgpkg.ApplyStates(w.orgs, mergedStates, func(org *Organization) {
		_ = w.orgTreasury(org)
	})
	transferorgpkg.ReconcileAgentsOrg(w.agents, w.orgs, ownerByAgent)
}
