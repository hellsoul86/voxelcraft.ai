package world

import "voxelcraft.ai/internal/protocol"

type JoinRequest struct {
	Name        string
	DeltaVoxels bool
	Out         chan []byte
	Resp        chan JoinResponse
}

type AttachRequest struct {
	ResumeToken string
	DeltaVoxels bool
	Out         chan []byte
	Resp        chan JoinResponse
}

type JoinResponse struct {
	Welcome  protocol.WelcomeMsg
	Catalogs []protocol.CatalogMsg
}

type ActionEnvelope struct {
	AgentID string
	Act     protocol.ActMsg
}

type RecordedJoin struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
}

type TickLogger interface {
	WriteTick(entry TickLogEntry) error
}

type AuditLogger interface {
	WriteAudit(entry AuditEntry) error
}

type TickLogEntry struct {
	Tick    uint64           `json:"tick"`
	Joins   []RecordedJoin   `json:"joins,omitempty"`
	Leaves  []string         `json:"leaves,omitempty"`
	Actions []RecordedAction `json:"actions,omitempty"`
	Digest  string           `json:"digest"`
}

type RecordedAction struct {
	AgentID string          `json:"agent_id"`
	Act     protocol.ActMsg `json:"act"`
}

type AuditEntry struct {
	Tick    uint64         `json:"tick"`
	Actor   string         `json:"actor"`
	Action  string         `json:"action"` // e.g. "SET_BLOCK"
	Pos     [3]int         `json:"pos"`
	From    uint16         `json:"from"`
	To      uint16         `json:"to"`
	Reason  string         `json:"reason,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}
