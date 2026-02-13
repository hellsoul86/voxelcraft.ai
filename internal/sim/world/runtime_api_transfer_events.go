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
	Resp    chan error
}

func (w *World) RequestEventsAfter(ctx context.Context, agentID string, sinceCursor uint64, limit int) ([]EventCursorItem, uint64, error) {
	if w == nil {
		return nil, sinceCursor, errors.New("event query not available")
	}
	return transfereventspkg.RequestAfter(ctx, w.eventsReq, agentID, sinceCursor, limit)
}

func (w *World) RequestInjectEvent(ctx context.Context, agentID string, ev protocol.Event) error {
	if w == nil {
		return errors.New("event injection not available")
	}
	req := injectEventReq{
		AgentID: agentID,
		Event:   ev,
		Resp:    make(chan error, 1),
	}
	select {
	case w.injectEvent <- req:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-req.Resp:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *World) handleEventsReq(req transfereventspkg.Req) {
	resp := transferruntimepkg.HandleEventsReq(req, func(agentID string, sinceCursor uint64, limit int) ([]transfereventspkg.CursorItem, uint64, bool) {
		a := w.agents[agentID]
		if a == nil {
			return nil, sinceCursor, false
		}
		logItems, next := a.EventsAfter(sinceCursor, limit)
		items := make([]transfereventspkg.CursorItem, 0, len(logItems))
		for _, it := range logItems {
			items = append(items, transfereventspkg.CursorItem{
				Cursor: it.Cursor,
				Event:  it.Event,
			})
		}
		return items, next, true
	})
	if req.Resp == nil {
		return
	}
	select {
	case req.Resp <- resp:
	default:
	}
}
