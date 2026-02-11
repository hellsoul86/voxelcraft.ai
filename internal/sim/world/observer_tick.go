package world

import (
	"encoding/json"

	"voxelcraft.ai/internal/observerproto"
)

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

type observerClient struct {
	id      string
	tickOut chan []byte
	dataOut chan []byte

	cfg observerCfg

	// Chunks tracked for this observer (may be pending full send).
	chunks map[ChunkKey]*observerChunk

	// Voxel chunks tracked for this observer (for 3D rendering).
	voxelChunks map[ChunkKey]*observerVoxelChunk
}

type observerCfg struct {
	chunkRadius int
	maxChunks   int

	focusAgentID   string
	voxelRadius    int
	voxelMaxChunks int
}

type observerChunk struct {
	key ChunkKey

	// lastWantedTick is updated whenever the chunk is in the current wanted set.
	lastWantedTick uint64

	// sentFull indicates we have enqueued at least one CHUNK_SURFACE for this chunk.
	sentFull bool

	// needsFull forces a CHUNK_SURFACE resend (e.g. after a dropped PATCH).
	needsFull bool

	// surface is the last known surface state for the chunk, used for PATCH diffs.
	// Populated lazily when we send (or attempt to send) a full surface.
	surface []surfaceCell // len=256
}

type surfaceCell struct {
	b uint16
	y uint8
}

type observerVoxelChunk struct {
	key ChunkKey

	lastWantedTick uint64

	sentFull  bool
	needsFull bool

	// blocks is the last known chunk block array, used for patch updates.
	// Populated lazily when we send (or attempt to send) a full chunk.
	blocks []uint16 // len = 16*16*height
}

const (
	observerEvictAfterTicks      = 50
	observerMaxFullChunksPerTick = 32

	observerVoxelEvictAfterTicks      = 10
	observerMaxFullVoxelChunksPerTick = 8
)

func (w *World) stepObservers(nowTick uint64, joins []RecordedJoin, leaves []string, actions []RecordedAction) {
	if w == nil || len(w.observers) == 0 {
		return
	}

	// Snapshot agent list (sorted for stable output).
	agentsSorted := w.sortedAgents()
	agentStates := make([]observerproto.AgentState, 0, len(agentsSorted))

	connectedChunks := make([]ChunkKey, 0, len(agentsSorted))
	for _, a := range agentsSorted {
		if a == nil {
			continue
		}
		connected := w.clients[a.ID] != nil
		if connected {
			cx := floorDiv(a.Pos.X, 16)
			cz := floorDiv(a.Pos.Z, 16)
			connectedChunks = append(connectedChunks, ChunkKey{CX: cx, CZ: cz})
		}

		st := observerproto.AgentState{
			ID:           a.ID,
			Name:         a.Name,
			Connected:    connected,
			OrgID:        a.OrgID,
			Pos:          a.Pos.ToArray(),
			Yaw:          a.Yaw,
			HP:           a.HP,
			Hunger:       a.Hunger,
			StaminaMilli: a.StaminaMilli,
		}
		if a.MoveTask != nil {
			st.MoveTask = w.observerMoveTaskState(a, nowTick)
		}
		if a.WorkTask != nil {
			st.WorkTask = w.observerWorkTaskState(a)
		}
		agentStates = append(agentStates, st)
	}

	joinsOut := make([]observerproto.JoinInfo, 0, len(joins))
	for _, j := range joins {
		joinsOut = append(joinsOut, observerproto.JoinInfo{AgentID: j.AgentID, Name: j.Name})
	}

	actionsOut := make([]observerproto.RecordedAction, 0, len(actions))
	for _, a := range actions {
		actionsOut = append(actionsOut, observerproto.RecordedAction{AgentID: a.AgentID, Act: a.Act})
	}

	auditsOut := make([]observerproto.AuditEntry, 0, len(w.obsAuditsThisTick))
	for _, e := range w.obsAuditsThisTick {
		auditsOut = append(auditsOut, observerproto.AuditEntry{
			Tick:   e.Tick,
			Actor:  e.Actor,
			Action: e.Action,
			Pos:    e.Pos,
			From:   e.From,
			To:     e.To,
			Reason: e.Reason,
		})
	}

	// Chunk surfaces / patches are per-observer (depends on subscription settings).
	for _, c := range w.observers {
		w.stepObserverChunksForClient(nowTick, c, connectedChunks, w.obsAuditsThisTick)
		w.stepObserverVoxelChunksForClient(nowTick, c, w.obsAuditsThisTick)
	}

	msg := observerproto.TickMsg{
		Type:                "TICK",
		ProtocolVersion:     observerproto.Version,
		Tick:                nowTick,
		TimeOfDay:           w.timeOfDay(nowTick),
		Weather:             w.weather,
		ActiveEventID:       w.activeEventID,
		ActiveEventEndsTick: w.activeEventEnds,
		Agents:              agentStates,
		Joins:               joinsOut,
		Leaves:              leaves,
		Actions:             actionsOut,
		Audits:              auditsOut,
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	for _, c := range w.observers {
		sendLatest(c.tickOut, b)
	}
}
