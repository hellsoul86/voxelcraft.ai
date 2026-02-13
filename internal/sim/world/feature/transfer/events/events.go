package events

import (
	"context"
	"errors"

	"voxelcraft.ai/internal/protocol"
)

type CursorItem struct {
	Cursor uint64
	Event  protocol.Event
}

type Req struct {
	AgentID     string
	SinceCursor uint64
	Limit       int
	Resp        chan Resp
}

type Resp struct {
	Items      []CursorItem
	NextCursor uint64
	Err        string
}

func BuildResp(agentFound bool, sinceCursor uint64, items []CursorItem, nextCursor uint64) Resp {
	resp := Resp{NextCursor: sinceCursor}
	if !agentFound {
		resp.Err = "agent not found"
		return resp
	}
	if len(items) > 0 {
		resp.Items = make([]CursorItem, len(items))
		copy(resp.Items, items)
	}
	if nextCursor >= sinceCursor {
		resp.NextCursor = nextCursor
	}
	return resp
}

func RequestAfter(ctx context.Context, ch chan<- Req, agentID string, sinceCursor uint64, limit int) ([]CursorItem, uint64, error) {
	if ch == nil {
		return nil, sinceCursor, errors.New("event query not available")
	}
	req := Req{
		AgentID:     agentID,
		SinceCursor: sinceCursor,
		Limit:       limit,
		Resp:        make(chan Resp, 1),
	}
	select {
	case ch <- req:
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
