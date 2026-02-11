package protocol

// EVENT_BATCH_REQ (client -> server)
type EventBatchReqMsg struct {
	Type            string `json:"type"`
	ProtocolVersion string `json:"protocol_version"`
	ReqID           string `json:"req_id"`
	SinceCursor     uint64 `json:"since_cursor"`
	Limit           int    `json:"limit"`
}

type EventBatchItem struct {
	Cursor uint64 `json:"cursor"`
	Event  Event  `json:"event"`
}

// EVENT_BATCH (server -> client)
type EventBatchMsg struct {
	Type            string           `json:"type"`
	ProtocolVersion string           `json:"protocol_version"`
	ReqID           string           `json:"req_id"`
	Events          []EventBatchItem `json:"events"`
	NextCursor      uint64           `json:"next_cursor"`
	WorldID         string           `json:"world_id,omitempty"`
}
