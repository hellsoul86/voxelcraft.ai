package events

import "voxelcraft.ai/internal/protocol"

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
