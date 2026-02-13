package world

import "voxelcraft.ai/internal/protocol"

import modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
import transferruntimepkg "voxelcraft.ai/internal/sim/world/feature/transfer/runtime"

type Vec3i = modelpkg.Vec3i
type Sign = modelpkg.Sign
type ConveyorMeta = modelpkg.ConveyorMeta
type FunScore = modelpkg.FunScore
type Equipment = modelpkg.Equipment
type Agent = modelpkg.Agent
type Container = modelpkg.Container
type ClaimFlags = modelpkg.ClaimFlags
type LandClaim = modelpkg.LandClaim
type OrgKind = modelpkg.OrgKind
type OrgRole = modelpkg.OrgRole
type Organization = modelpkg.Organization
type ContractState = modelpkg.ContractState
type Contract = modelpkg.Contract
type ItemEntity = modelpkg.ItemEntity
type Structure = modelpkg.Structure
type MemoryEntry = modelpkg.MemoryEntry
type RateWindowSnapshot = modelpkg.RateWindowSnapshot
type FunDecaySnapshot = modelpkg.FunDecaySnapshot
type AgentTransfer = transferruntimepkg.AgentTransfer
type OrgTransfer = transferruntimepkg.OrgTransfer

const (
	ClaimTypeDefault   = modelpkg.ClaimTypeDefault
	ClaimTypeHomestead = modelpkg.ClaimTypeHomestead
	ClaimTypeCityCore  = modelpkg.ClaimTypeCityCore
)

const (
	OrgGuild OrgKind = modelpkg.OrgGuild
	OrgCity  OrgKind = modelpkg.OrgCity
)

const (
	OrgLeader  OrgRole = modelpkg.OrgLeader
	OrgOfficer OrgRole = modelpkg.OrgOfficer
	OrgMember  OrgRole = modelpkg.OrgMember
)

const (
	ContractOpen      ContractState = modelpkg.ContractOpen
	ContractAccepted  ContractState = modelpkg.ContractAccepted
	ContractCompleted ContractState = modelpkg.ContractCompleted
	ContractFailed    ContractState = modelpkg.ContractFailed
)

func Manhattan(a, b Vec3i) int {
	return modelpkg.Manhattan(a, b)
}

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

// ObserverJoinRequest registers a read-only observer session that receives:
// - chunk surface tiles (dataOut)
// - per-tick global state (tickOut)
//
// All observer state is maintained by the world loop goroutine.
type ObserverJoinRequest struct {
	SessionID string
	TickOut   chan []byte
	DataOut   chan []byte

	ChunkRadius int
	MaxChunks   int

	// Optional: stream 3D voxels around a focused agent.
	FocusAgentID   string
	VoxelRadius    int
	VoxelMaxChunks int
}

// ObserverSubscribeRequest updates an existing observer session subscription settings.
type ObserverSubscribeRequest struct {
	SessionID string

	ChunkRadius int
	MaxChunks   int

	// Optional: stream 3D voxels around a focused agent.
	FocusAgentID   string
	VoxelRadius    int
	VoxelMaxChunks int
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
