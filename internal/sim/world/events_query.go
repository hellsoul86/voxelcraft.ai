package world

import (
	"context"
	"errors"

	"voxelcraft.ai/internal/protocol"
)

type EventCursorItem struct {
	Cursor uint64
	Event  protocol.Event
}

type eventsReq struct {
	AgentID     string
	SinceCursor uint64
	Limit       int
	Resp        chan eventsResp
}

type eventsResp struct {
	Items      []EventCursorItem
	NextCursor uint64
	Err        string
}

func (w *World) RequestEventsAfter(ctx context.Context, agentID string, sinceCursor uint64, limit int) ([]EventCursorItem, uint64, error) {
	if w == nil || w.eventsReq == nil {
		return nil, sinceCursor, errors.New("event query not available")
	}
	req := eventsReq{
		AgentID:     agentID,
		SinceCursor: sinceCursor,
		Limit:       limit,
		Resp:        make(chan eventsResp, 1),
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

func (w *World) handleEventsReq(req eventsReq) {
	resp := eventsResp{NextCursor: req.SinceCursor}
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
	items, next := a.EventsAfter(req.SinceCursor, req.Limit)
	resp.Items = make([]EventCursorItem, 0, len(items))
	for _, it := range items {
		resp.Items = append(resp.Items, EventCursorItem{Cursor: it.Cursor, Event: it.Event})
	}
	resp.NextCursor = next
}
