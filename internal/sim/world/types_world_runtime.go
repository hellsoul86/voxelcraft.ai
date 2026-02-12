package world

import (
	"sync/atomic"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/catalogs"
	transfereventspkg "voxelcraft.ai/internal/sim/world/feature/transfer/events"
	transferruntimepkg "voxelcraft.ai/internal/sim/world/feature/transfer/runtime"
	modelpkg "voxelcraft.ai/internal/sim/world/kernel/model"
)

type Trade = modelpkg.Trade
type Board = modelpkg.Board
type BoardPost = modelpkg.BoardPost

// World is a single-threaded authoritative simulation.
// All state must be accessed only from the world loop goroutine.
type World struct {
	cfg      WorldConfig
	catalogs *catalogs.Catalogs

	tick    atomic.Uint64
	metrics atomic.Value

	chunks *ChunkStore

	// Derived from catalogs at startup; does not affect determinism/digests directly.
	smeltByInput map[string]catalogs.RecipeDef

	agents  map[string]*Agent
	clients map[string]*clientState

	claims     map[string]*LandClaim
	containers map[Vec3i]*Container
	items      map[string]*ItemEntity
	itemsAt    map[Vec3i][]string // pos -> item entity ids (in insertion order)
	conveyors  map[Vec3i]ConveyorMeta
	switches   map[Vec3i]bool
	trades     map[string]*Trade
	boards     map[string]*Board
	signs      map[Vec3i]*Sign
	contracts  map[string]*Contract
	laws       map[string]*Law
	orgs       map[string]*Organization

	inbox         chan ActionEnvelope
	join          chan JoinRequest
	attach        chan AttachRequest
	admin         chan adminSnapshotReq
	adminReset    chan adminResetReq
	agentPosReq   chan transferruntimepkg.AgentPosReq
	eventsReq     chan transfereventspkg.Req
	actDedupeReq  chan actDedupeReq
	orgMetaReq    chan transferruntimepkg.OrgMetaReq
	orgMetaUpsert chan transferruntimepkg.OrgMetaUpsertReq
	leave         chan string
	stop          chan struct{}
	transferOut   chan transferOutReq
	transferIn    chan transferInReq
	injectEvent   chan injectEventReq

	// Observer (admin-only, read-only)
	observerJoin  chan ObserverJoinRequest
	observerSub   chan ObserverSubscribeRequest
	observerLeave chan string

	nextAgentNum    atomic.Uint64
	nextTaskNum     atomic.Uint64
	nextLandNum     atomic.Uint64
	nextTradeNum    atomic.Uint64
	nextPostNum     atomic.Uint64
	nextContractNum atomic.Uint64
	nextLawNum      atomic.Uint64
	nextOrgNum      atomic.Uint64
	nextItemNum     atomic.Uint64

	// Optional loggers (may be nil). Implemented in internal/persistence/*.
	tickLogger  TickLogger
	auditLogger AuditLogger

	// Optional snapshot sink (may be nil). Snapshot writing should be off-thread.
	snapshotSink chan<- snapshot.SnapshotV1

	// World director (MVP): a single active event + simple weather override.
	weather           string
	weatherUntilTick  uint64
	activeEventID     string
	activeEventStart  uint64
	activeEventEnds   uint64
	activeEventCenter Vec3i
	activeEventRadius int

	stats *WorldStats

	// Fun-score: track blueprinted structures for delayed awards + usage/influence.
	structures map[string]*Structure

	// Observer sessions (admin-only, read-only).
	observers map[string]*observerClient
	// Per-tick audit events for observers (captured via auditSetBlock).
	obsAuditsThisTick []AuditEntry

	resetTotal uint64

	resourceDensity     map[string]float64
	nextDensitySampleAt uint64

	actDedupe map[actDedupeKey]actDedupeEntry
}

type clientState struct {
	Out         chan []byte
	DeltaVoxels bool
	LastVoxels  []uint16
}
