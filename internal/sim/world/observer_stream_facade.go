package world

import (
	"strings"

	"voxelcraft.ai/internal/observerproto"
	boardspkg "voxelcraft.ai/internal/sim/world/feature/observer/boards"
	chunkspkg "voxelcraft.ai/internal/sim/world/feature/observer/chunks"
	observerruntimepkg "voxelcraft.ai/internal/sim/world/feature/observer/runtime"
	streamspkg "voxelcraft.ai/internal/sim/world/feature/observer/stream"
)

type observerClient = streamspkg.Client
type surfaceCell = chunkspkg.SurfaceCell

func (w *World) newPostID() string {
	return boardspkg.NewPostID(w.nextPostNum.Add(1))
}

func boardIDAt(pos Vec3i) string {
	return boardspkg.BoardIDAt(pos)
}

func (w *World) ensureBoard(pos Vec3i) *Board {
	return boardspkg.EnsureBoard(w.boards, pos)
}

func (w *World) removeBoard(pos Vec3i) {
	boardspkg.RemoveBoard(w.boards, pos)
}

func (w *World) handleObserverJoin(req ObserverJoinRequest) {
	if w == nil {
		return
	}
	observerruntimepkg.JoinSession(w.observers, observerruntimepkg.JoinRequest{
		SessionID:      req.SessionID,
		TickOut:        req.TickOut,
		DataOut:        req.DataOut,
		ChunkRadius:    req.ChunkRadius,
		MaxChunks:      req.MaxChunks,
		FocusAgentID:   req.FocusAgentID,
		VoxelRadius:    req.VoxelRadius,
		VoxelMaxChunks: req.VoxelMaxChunks,
	})
}

func (w *World) handleObserverSubscribe(req ObserverSubscribeRequest) {
	if w == nil {
		return
	}
	observerruntimepkg.SubscribeSession(w.observers, observerruntimepkg.SubscribeRequest{
		SessionID:      req.SessionID,
		ChunkRadius:    req.ChunkRadius,
		MaxChunks:      req.MaxChunks,
		FocusAgentID:   req.FocusAgentID,
		VoxelRadius:    req.VoxelRadius,
		VoxelMaxChunks: req.VoxelMaxChunks,
	})
}

func (w *World) handleObserverLeave(sessionID string) {
	if w == nil {
		return
	}
	observerruntimepkg.LeaveSession(w.observers, sessionID)
}

func (w *World) computeChunkSurface(cx, cz int) []surfaceCell {
	ch := w.chunkForSurface(cx, cz)
	return chunkspkg.ComputeChunkSurface(ch.Blocks, cx, cz, w.chunks.gen.Air, w.chunks.gen.BoundaryR)
}

func (w *World) computeChunkVoxels(cx, cz int) []uint16 {
	ch := w.chunkForVoxels(cx, cz)
	if ch == nil {
		return nil
	}
	return chunkspkg.ComputeChunkVoxels(ch.Blocks, cx, cz, w.chunks.gen.Air, w.chunks.gen.BoundaryR)
}

func (w *World) computeSurfaceCellAt(wx, wz int) surfaceCell {
	cell := chunkspkg.ComputeSurfaceCellAt(wx, wz, w.chunks.gen.Air, w.chunks.gen.BoundaryR, func(cx, cz int) []uint16 {
		ch := w.chunkForSurface(cx, cz)
		if ch == nil {
			return nil
		}
		return ch.Blocks
	})
	return cell
}

func (w *World) chunkForSurface(cx, cz int) *Chunk {
	if w == nil || w.chunks == nil {
		return &Chunk{CX: cx, CZ: cz, Blocks: nil}
	}
	k := ChunkKey{CX: cx, CZ: cz}
	if ch, ok := w.chunks.chunks[k]; ok && ch != nil {
		return ch
	}
	// Generate an ephemeral chunk without mutating the world's loaded chunk set. This ensures
	// observer clients cannot affect simulation state/digests by "viewing" far-away terrain.
	tmp := &Chunk{
		CX:     cx,
		CZ:     cz,
		Blocks: make([]uint16, 16*16),
	}
	w.chunks.generateChunk(tmp)
	return tmp
}

func (w *World) chunkForVoxels(cx, cz int) *Chunk {
	// Same semantics as chunkForSurface; kept separate to make intent explicit.
	return w.chunkForSurface(cx, cz)
}

func trySend(ch chan []byte, b []byte) bool {
	select {
	case ch <- b:
		return true
	default:
		return false
	}
}

func (w *World) stepObserverChunksForClient(nowTick uint64, c *observerClient, connected []ChunkKey, audits []AuditEntry) {
	if w == nil || c == nil {
		return
	}
	connectedIn := make([]streamspkg.ChunkKey, 0, len(connected))
	for _, k := range connected {
		connectedIn = append(connectedIn, streamspkg.ChunkKey{CX: k.CX, CZ: k.CZ})
	}
	auditsIn := make([]streamspkg.AuditEntry, 0, len(audits))
	for _, a := range audits {
		auditsIn = append(auditsIn, streamspkg.AuditEntry{
			Action: a.Action,
			Pos:    a.Pos,
			To:     a.To,
		})
	}

	streamspkg.StepChunkClient(c, streamspkg.ChunkClientInput{
		NowTick:   nowTick,
		Connected: connectedIn,
		Audits:    auditsIn,
	}, streamspkg.ChunkClientDeps{
		ComputeSurface: func(cx, cz int) []chunkspkg.SurfaceCell {
			return w.computeChunkSurface(cx, cz)
		},
		ComputeSurfaceAt: func(wx, wz int) chunkspkg.SurfaceCell {
			return w.computeSurfaceCellAt(wx, wz)
		},
		SendFull: func(key streamspkg.ChunkKey, surface []chunkspkg.SurfaceCell) bool {
			return w.sendChunkSurfaceRaw(c, ChunkKey{CX: key.CX, CZ: key.CZ}, surface)
		},
		SendPatch: func(key streamspkg.ChunkKey, cells []streamspkg.ChunkPatchCell) bool {
			out := make([]observerproto.ChunkPatchCell, 0, len(cells))
			for _, cell := range cells {
				out = append(out, observerproto.ChunkPatchCell{
					X: cell.X, Z: cell.Z, Y: cell.Y, Block: cell.Block,
				})
			}
			b, err := streamspkg.BuildChunkPatchMsg(key.CX, key.CZ, out)
			if err != nil {
				return false
			}
			return trySend(c.DataOut, b)
		},
		SendEvict: func(key streamspkg.ChunkKey) bool {
			b, err := streamspkg.BuildChunkEvictMsg(key.CX, key.CZ)
			if err != nil {
				return false
			}
			return trySend(c.DataOut, b)
		},
	})
}

func (w *World) sendChunkSurfaceRaw(c *observerClient, key ChunkKey, surface []surfaceCell) bool {
	if w == nil || c == nil || surface == nil {
		return false
	}
	blocks := make([]uint16, len(surface))
	ys := make([]byte, len(surface))
	for i, cell := range surface {
		blocks[i] = cell.B
		ys[i] = cell.Y
	}
	data := streamspkg.EncodeSurfacePAL16Y8(blocks, ys)
	b, err := streamspkg.BuildChunkSurfaceMsg(key.CX, key.CZ, data)
	if err != nil {
		return false
	}
	return trySend(c.DataOut, b)
}

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
			st.MoveTask = observerruntimepkg.BuildMoveTaskStateFromWorld(a, func(id string) (Vec3i, bool) {
				return w.followTargetPos(id)
			})
		}
		if a.WorkTask != nil {
			st.WorkTask = observerruntimepkg.BuildWorkTaskStateFromWorld(a, w.workProgressForAgent(a, a.WorkTask))
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

	b, err := streamspkg.BuildTickMsgBytes(streamspkg.TickBuildInput{
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
	})
	if err != nil {
		return
	}
	for _, c := range w.observers {
		sendLatest(c.TickOut, b)
	}
}

func (w *World) stepObserverVoxelChunksForClient(nowTick uint64, c *observerClient, audits []AuditEntry) {
	if w == nil || c == nil {
		return
	}

	focusID := strings.TrimSpace(c.Config.FocusAgentID)
	enabled := focusID != "" && c.Config.VoxelRadius > 0
	centers := []streamspkg.ChunkKey{}
	if enabled {
		if a := w.agents[focusID]; a != nil {
			centers = append(centers, streamspkg.ChunkKey{
				CX: floorDiv(a.Pos.X, 16),
				CZ: floorDiv(a.Pos.Z, 16),
			})
		}
	}

	auditsIn := make([]streamspkg.AuditEntry, 0, len(audits))
	for _, a := range audits {
		auditsIn = append(auditsIn, streamspkg.AuditEntry{
			Action: a.Action,
			Pos:    a.Pos,
			To:     a.To,
		})
	}

	streamspkg.StepVoxelClient(c, streamspkg.VoxelClientInput{
		NowTick:      nowTick,
		Enabled:      enabled,
		FocusCenters: centers,
		Audits:       auditsIn,
	}, streamspkg.VoxelClientDeps{
		ComputeVoxels: func(cx, cz int) []uint16 {
			return w.computeChunkVoxels(cx, cz)
		},
		SendFull: func(key streamspkg.ChunkKey, blocks []uint16) bool {
			return w.sendChunkVoxelsRaw(c, ChunkKey{CX: key.CX, CZ: key.CZ}, blocks)
		},
		SendPatch: func(key streamspkg.ChunkKey, cells []streamspkg.VoxelPatchCell) bool {
			out := make([]observerproto.ChunkVoxelPatchCell, 0, len(cells))
			for _, cell := range cells {
				out = append(out, observerproto.ChunkVoxelPatchCell{
					X: cell.X, Y: cell.Y, Z: cell.Z, Block: cell.Block,
				})
			}
			b, err := streamspkg.BuildChunkVoxelPatchMsg(key.CX, key.CZ, out)
			if err != nil {
				return false
			}
			return trySend(c.DataOut, b)
		},
		SendEvict: func(key streamspkg.ChunkKey) bool {
			b, err := streamspkg.BuildChunkVoxelsEvictMsg(key.CX, key.CZ)
			if err != nil {
				return false
			}
			return trySend(c.DataOut, b)
		},
	})
}

func (w *World) sendChunkVoxelsRaw(c *observerClient, key ChunkKey, blocks []uint16) bool {
	if w == nil || c == nil || blocks == nil {
		return false
	}
	data := streamspkg.EncodeVoxelsPAL16U16LE(blocks)
	b, err := streamspkg.BuildChunkVoxelsMsg(key.CX, key.CZ, data)
	if err != nil {
		return false
	}
	return trySend(c.DataOut, b)
}
