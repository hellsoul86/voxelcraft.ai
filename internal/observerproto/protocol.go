package observerproto

import "voxelcraft.ai/internal/protocol"

// Version is the observer protocol version (separate from the agent WS protocol).
const Version = "0.1"

// Client -> Server. First message on the observer WS connection, and can be re-sent to update settings.
type SubscribeMsg struct {
	Type            string `json:"type"`
	ProtocolVersion string `json:"protocol_version"`
	ChunkRadius     int    `json:"chunk_radius"`
	MaxChunks       int    `json:"max_chunks"`

	// Optional: request 3D voxels around a focused agent (for true 3D rendering).
	FocusAgentID   string `json:"focus_agent_id,omitempty"`
	VoxelRadius    int    `json:"voxel_radius,omitempty"`
	VoxelMaxChunks int    `json:"voxel_max_chunks,omitempty"`
}

// HTTP response for GET /admin/v1/observer/bootstrap.
type BootstrapResponse struct {
	ProtocolVersion string      `json:"protocol_version"`
	WorldID         string      `json:"world_id"`
	Tick            uint64      `json:"tick"`
	WorldParams     WorldParams `json:"world_params"`
	BlockPalette    []string    `json:"block_palette"`
}

type WorldParams struct {
	TickRateHz int    `json:"tick_rate_hz"`
	ChunkSize  [3]int `json:"chunk_size"`
	Height     int    `json:"height"`
	Seed       int64  `json:"seed"`
	BoundaryR  int    `json:"boundary_r"`
}

// Server -> Client. Sent every tick.
type TickMsg struct {
	Type            string `json:"type"`
	ProtocolVersion string `json:"protocol_version"`
	Tick            uint64 `json:"tick"`

	TimeOfDay float64 `json:"time_of_day"`
	Weather   string  `json:"weather"`

	ActiveEventID       string `json:"active_event_id,omitempty"`
	ActiveEventEndsTick uint64 `json:"active_event_ends_tick,omitempty"`

	Agents  []AgentState     `json:"agents"`
	Joins   []JoinInfo       `json:"joins,omitempty"`
	Leaves  []string         `json:"leaves,omitempty"`
	Actions []RecordedAction `json:"actions,omitempty"`
	Audits  []AuditEntry     `json:"audits,omitempty"`
}

type JoinInfo struct {
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
}

type RecordedAction struct {
	AgentID string          `json:"agent_id"`
	Act     protocol.ActMsg `json:"act"`
}

type AuditEntry struct {
	Tick   uint64 `json:"tick"`
	Actor  string `json:"actor"`
	Action string `json:"action"`
	Pos    [3]int `json:"pos"`
	From   uint16 `json:"from"`
	To     uint16 `json:"to"`
	Reason string `json:"reason,omitempty"`
}

type AgentState struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Connected bool   `json:"connected"`
	OrgID     string `json:"org_id,omitempty"`

	Pos [3]int `json:"pos"`
	Yaw int    `json:"yaw"`

	HP           int `json:"hp"`
	Hunger       int `json:"hunger"`
	StaminaMilli int `json:"stamina_milli"`

	MoveTask *TaskState `json:"move_task,omitempty"`
	WorkTask *TaskState `json:"work_task,omitempty"`
}

type TaskState struct {
	Kind     string  `json:"kind"`
	TargetID string  `json:"target_id,omitempty"`
	Target   [3]int  `json:"target,omitempty"`
	Progress float64 `json:"progress"`
	EtaTicks int     `json:"eta_ticks,omitempty"`
}

// Server -> Client. Full 16x16 surface tile for a chunk.
type ChunkSurfaceMsg struct {
	Type            string `json:"type"`
	ProtocolVersion string `json:"protocol_version"`
	CX              int    `json:"cx"`
	CZ              int    `json:"cz"`
	Encoding        string `json:"encoding"`
	Data            string `json:"data"`
}

// Server -> Client. Patch cells for a chunk surface.
type ChunkPatchMsg struct {
	Type            string           `json:"type"`
	ProtocolVersion string           `json:"protocol_version"`
	CX              int              `json:"cx"`
	CZ              int              `json:"cz"`
	Cells           []ChunkPatchCell `json:"cells"`
}

type ChunkPatchCell struct {
	X     int    `json:"x"`
	Z     int    `json:"z"`
	Block uint16 `json:"block"`
	Y     int    `json:"y"`
}

// Server -> Client. Evict a chunk from the client cache.
type ChunkEvictMsg struct {
	Type            string `json:"type"`
	ProtocolVersion string `json:"protocol_version"`
	CX              int    `json:"cx"`
	CZ              int    `json:"cz"`
}

// Server -> Client. Full voxel data for a chunk (16x16xheight).
// Encoding "PAL16_U16LE_YZX" means:
// - Decode base64 to bytes, interpret as little-endian uint16 palette ids
// - Iteration order: for y in 0..height-1, for z in 0..15, for x in 0..15 (x fastest)
// - Total length: 16*16*height uint16s
type ChunkVoxelsMsg struct {
	Type            string `json:"type"`
	ProtocolVersion string `json:"protocol_version"`
	CX              int    `json:"cx"`
	CZ              int    `json:"cz"`
	Encoding        string `json:"encoding"`
	Data            string `json:"data"`
}

type ChunkVoxelPatchMsg struct {
	Type            string                `json:"type"`
	ProtocolVersion string                `json:"protocol_version"`
	CX              int                   `json:"cx"`
	CZ              int                   `json:"cz"`
	Cells           []ChunkVoxelPatchCell `json:"cells"`
}

type ChunkVoxelPatchCell struct {
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Z     int    `json:"z"`
	Block uint16 `json:"block"`
}

// Server -> Client. Evict voxel data for a chunk from the client cache.
type ChunkVoxelsEvictMsg struct {
	Type            string `json:"type"`
	ProtocolVersion string `json:"protocol_version"`
	CX              int    `json:"cx"`
	CZ              int    `json:"cz"`
}
