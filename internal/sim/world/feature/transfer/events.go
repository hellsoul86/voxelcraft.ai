package transfer

import "voxelcraft.ai/internal/protocol"

type EventCursorItem struct {
	Cursor uint64
	Event  protocol.Event
}

type EventsReq struct {
	AgentID     string
	SinceCursor uint64
	Limit       int
	Resp        chan EventsResp
}

type EventsResp struct {
	Items      []EventCursorItem
	NextCursor uint64
	Err        string
}
