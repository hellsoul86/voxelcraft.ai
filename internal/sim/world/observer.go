package world

import (
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"

	"voxelcraft.ai/internal/observerproto"
	"voxelcraft.ai/internal/sim/tasks"
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

func (w *World) Config() WorldConfig {
	if w == nil {
		return WorldConfig{}
	}
	cfg := w.cfg
	if cfg.MaintenanceCost != nil {
		m := make(map[string]int, len(cfg.MaintenanceCost))
		for k, v := range cfg.MaintenanceCost {
			m[k] = v
		}
		cfg.MaintenanceCost = m
	}
	return cfg
}

func (w *World) BlockPalette() []string {
	if w == nil || w.catalogs == nil {
		return nil
	}
	p := w.catalogs.Blocks.Palette
	out := make([]string, len(p))
	copy(out, p)
	return out
}

func (w *World) handleObserverJoin(req ObserverJoinRequest) {
	if w == nil || req.SessionID == "" || req.TickOut == nil || req.DataOut == nil {
		return
	}

	cfg := observerCfg{
		chunkRadius:    clampInt(req.ChunkRadius, 1, 32, 6),
		maxChunks:      clampInt(req.MaxChunks, 1, 16384, 1024),
		focusAgentID:   strings.TrimSpace(req.FocusAgentID),
		voxelRadius:    clampInt(req.VoxelRadius, 0, 8, 0),
		voxelMaxChunks: clampInt(req.VoxelMaxChunks, 1, 2048, 256),
	}

	// Replace existing session id if any (defensive).
	if old := w.observers[req.SessionID]; old != nil {
		close(old.tickOut)
		close(old.dataOut)
	}

	w.observers[req.SessionID] = &observerClient{
		id:          req.SessionID,
		tickOut:     req.TickOut,
		dataOut:     req.DataOut,
		cfg:         cfg,
		chunks:      map[ChunkKey]*observerChunk{},
		voxelChunks: map[ChunkKey]*observerVoxelChunk{},
	}
}

func (w *World) handleObserverSubscribe(req ObserverSubscribeRequest) {
	c := w.observers[req.SessionID]
	if c == nil {
		return
	}
	c.cfg.chunkRadius = clampInt(req.ChunkRadius, 1, 32, c.cfg.chunkRadius)
	c.cfg.maxChunks = clampInt(req.MaxChunks, 1, 16384, c.cfg.maxChunks)

	// Voxel streaming config. Allow disabling by sending voxel_radius=0.
	c.cfg.focusAgentID = strings.TrimSpace(req.FocusAgentID)
	if req.VoxelRadius < 0 {
		c.cfg.voxelRadius = 0
	} else if req.VoxelRadius > 8 {
		c.cfg.voxelRadius = 8
	} else {
		c.cfg.voxelRadius = req.VoxelRadius
	}
	if req.VoxelMaxChunks > 0 {
		c.cfg.voxelMaxChunks = clampInt(req.VoxelMaxChunks, 1, 2048, c.cfg.voxelMaxChunks)
	}
}

func (w *World) handleObserverLeave(sessionID string) {
	if sessionID == "" {
		return
	}
	c := w.observers[sessionID]
	if c == nil {
		return
	}
	delete(w.observers, sessionID)
	close(c.tickOut)
	close(c.dataOut)
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

func (w *World) observerMoveTaskState(a *Agent, nowTick uint64) *observerproto.TaskState {
	if w == nil || a == nil || a.MoveTask == nil {
		return nil
	}
	mt := a.MoveTask

	target := v3FromTask(mt.Target)
	if mt.Kind == tasks.KindFollow {
		if t, ok := w.followTargetPos(mt.TargetID); ok {
			target = t
		}
		want := int(ceil(mt.Distance))
		if want < 1 {
			want = 1
		}
		d := distXZ(a.Pos, target)
		prog := 0.0
		if d <= want {
			prog = 1.0
		}
		eta := d - want
		if eta < 0 {
			eta = 0
		}
		return &observerproto.TaskState{
			Kind:     string(mt.Kind),
			TargetID: mt.TargetID,
			Target:   target.ToArray(),
			Progress: prog,
			EtaTicks: eta,
		}
	}

	start := v3FromTask(mt.StartPos)

	// Match the agent OBS semantics: completion is within tolerance, and progress/eta are based on effective XZ distance.
	want := int(ceil(mt.Tolerance))
	if want < 1 {
		want = 1
	}
	distStart := distXZ(start, target)
	distCur := distXZ(a.Pos, target)
	totalEff := distStart - want
	if totalEff < 0 {
		totalEff = 0
	}
	remEff := distCur - want
	if remEff < 0 {
		remEff = 0
	}
	prog := 1.0
	if totalEff > 0 {
		prog = float64(totalEff-remEff) / float64(totalEff)
		if prog < 0 {
			prog = 0
		} else if prog > 1 {
			prog = 1
		}
	}
	eta := remEff
	return &observerproto.TaskState{
		Kind:     string(mt.Kind),
		Target:   target.ToArray(),
		Progress: prog,
		EtaTicks: eta,
	}
}

func (w *World) observerWorkTaskState(a *Agent) *observerproto.TaskState {
	if a == nil || a.WorkTask == nil {
		return nil
	}
	wt := a.WorkTask
	return &observerproto.TaskState{
		Kind:     string(wt.Kind),
		Progress: w.workProgressForAgent(a, wt),
	}
}

func (w *World) stepObserverChunksForClient(nowTick uint64, c *observerClient, connected []ChunkKey, audits []AuditEntry) {
	if w == nil || c == nil {
		return
	}

	wantKeys := computeWantedChunks(connected, c.cfg.chunkRadius, c.cfg.maxChunks)
	wantSet := make(map[ChunkKey]struct{}, len(wantKeys))
	for _, k := range wantKeys {
		wantSet[k] = struct{}{}
	}

	// Track wanted chunks, enqueue full surfaces as needed (rate-limited).
	fullBudget := observerMaxFullChunksPerTick
	canSend := true
	for _, k := range wantKeys {
		st := c.chunks[k]
		if st == nil {
			st = &observerChunk{
				key:            k,
				lastWantedTick: nowTick,
				needsFull:      true,
			}
			c.chunks[k] = st
		} else {
			st.lastWantedTick = nowTick
		}

		if canSend && st.needsFull && fullBudget > 0 {
			if st.surface == nil {
				st.surface = w.computeChunkSurface(k.CX, k.CZ)
			}
			if w.sendChunkSurface(c, st) {
				st.sentFull = true
				st.needsFull = false
				fullBudget--
			} else {
				// Channel is likely full; don't spend more CPU on sends this tick.
				canSend = false
			}
		}
	}

	// Apply audits -> patch (or force resync).
	patches := map[ChunkKey]map[int]observerproto.ChunkPatchCell{}
	for _, e := range audits {
		if e.Action != "SET_BLOCK" {
			continue
		}
		wx := e.Pos[0]
		wz := e.Pos[2]
		cx := floorDiv(wx, 16)
		cz := floorDiv(wz, 16)
		key := ChunkKey{CX: cx, CZ: cz}
		st := c.chunks[key]
		if st == nil {
			continue
		}
		// If we don't have a baseline surface yet, just ignore PATCH; a future full send will catch up.
		if st.surface == nil {
			continue
		}
		lx := mod(wx, 16)
		lz := mod(wz, 16)
		idx := lz*16 + lx
		newCell := w.computeSurfaceCellAt(wx, wz)
		old := st.surface[idx]
		if old == newCell {
			continue
		}
		st.surface[idx] = newCell
		if st.needsFull {
			continue
		}
		m := patches[key]
		if m == nil {
			m = map[int]observerproto.ChunkPatchCell{}
			patches[key] = m
		}
		m[idx] = observerproto.ChunkPatchCell{X: lx, Z: lz, Block: newCell.b, Y: int(newCell.y)}
	}

	for key, m := range patches {
		st := c.chunks[key]
		if st == nil || st.needsFull {
			continue
		}
		cells := make([]observerproto.ChunkPatchCell, 0, len(m))
		for _, cell := range m {
			cells = append(cells, cell)
		}
		sort.Slice(cells, func(i, j int) bool {
			if cells[i].Z != cells[j].Z {
				return cells[i].Z < cells[j].Z
			}
			return cells[i].X < cells[j].X
		})
		msg := observerproto.ChunkPatchMsg{
			Type:            "CHUNK_PATCH",
			ProtocolVersion: observerproto.Version,
			CX:              key.CX,
			CZ:              key.CZ,
			Cells:           cells,
		}
		b, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		if !trySend(c.dataOut, b) {
			// Force a full resync next tick if PATCH is dropped.
			st.needsFull = true
		}
	}

	// Evictions.
	if len(c.chunks) > 0 {
		var evictKeys []ChunkKey
		for k, st := range c.chunks {
			if _, ok := wantSet[k]; ok {
				continue
			}
			if nowTick-st.lastWantedTick < observerEvictAfterTicks {
				continue
			}
			if !st.sentFull {
				evictKeys = append(evictKeys, k)
				continue
			}

			msg := observerproto.ChunkEvictMsg{
				Type:            "CHUNK_EVICT",
				ProtocolVersion: observerproto.Version,
				CX:              k.CX,
				CZ:              k.CZ,
			}
			b, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			if trySend(c.dataOut, b) {
				evictKeys = append(evictKeys, k)
			}
		}
		for _, k := range evictKeys {
			delete(c.chunks, k)
		}
	}
}

func (w *World) stepObserverVoxelChunksForClient(nowTick uint64, c *observerClient, audits []AuditEntry) {
	if w == nil || c == nil {
		return
	}

	focusID := strings.TrimSpace(c.cfg.focusAgentID)
	radius := c.cfg.voxelRadius
	if focusID == "" || radius <= 0 {
		// If voxels are disabled, evict any previously-sent voxel chunks immediately.
		if len(c.voxelChunks) > 0 {
			for k, st := range c.voxelChunks {
				if st != nil && st.sentFull {
					msg := observerproto.ChunkVoxelsEvictMsg{
						Type:            "CHUNK_VOXELS_EVICT",
						ProtocolVersion: observerproto.Version,
						CX:              k.CX,
						CZ:              k.CZ,
					}
					if b, err := json.Marshal(msg); err == nil {
						_ = trySend(c.dataOut, b)
					}
				}
				delete(c.voxelChunks, k)
			}
		}
		return
	}

	a := w.agents[focusID]
	if a == nil {
		return
	}

	center := ChunkKey{CX: floorDiv(a.Pos.X, 16), CZ: floorDiv(a.Pos.Z, 16)}
	wantKeys := computeWantedChunks([]ChunkKey{center}, radius, c.cfg.voxelMaxChunks)
	wantSet := make(map[ChunkKey]struct{}, len(wantKeys))
	for _, k := range wantKeys {
		wantSet[k] = struct{}{}
	}

	// Track wanted chunks and enqueue full voxel chunks as needed (rate-limited).
	fullBudget := observerMaxFullVoxelChunksPerTick
	canSend := true
	for _, k := range wantKeys {
		st := c.voxelChunks[k]
		if st == nil {
			st = &observerVoxelChunk{
				key:            k,
				lastWantedTick: nowTick,
				needsFull:      true,
			}
			c.voxelChunks[k] = st
		} else {
			st.lastWantedTick = nowTick
		}

		if canSend && st.needsFull && fullBudget > 0 {
			if st.blocks == nil {
				st.blocks = w.computeChunkVoxels(k.CX, k.CZ)
			}
			if w.sendChunkVoxels(c, st) {
				st.sentFull = true
				st.needsFull = false
				fullBudget--
			} else {
				// Channel is likely full; don't spend more CPU on sends this tick.
				canSend = false
			}
		}
	}

	// Apply audits -> voxel patch (or force resync).
	patches := map[ChunkKey]map[int]observerproto.ChunkVoxelPatchCell{}
	for _, e := range audits {
		if e.Action != "SET_BLOCK" {
			continue
		}
		wx, wy, wz := e.Pos[0], e.Pos[1], e.Pos[2]
		// 2D world: only y==0 exists.
		if wy != 0 {
			continue
		}
		cx := floorDiv(wx, 16)
		cz := floorDiv(wz, 16)
		key := ChunkKey{CX: cx, CZ: cz}
		if _, ok := wantSet[key]; !ok {
			continue
		}
		st := c.voxelChunks[key]
		if st == nil || st.blocks == nil {
			continue
		}
		if st.needsFull {
			continue
		}
		lx := mod(wx, 16)
		lz := mod(wz, 16)
		idx := lx + lz*16
		if idx < 0 || idx >= len(st.blocks) {
			continue
		}
		if st.blocks[idx] == e.To {
			continue
		}
		st.blocks[idx] = e.To

		m := patches[key]
		if m == nil {
			m = map[int]observerproto.ChunkVoxelPatchCell{}
			patches[key] = m
		}
		m[idx] = observerproto.ChunkVoxelPatchCell{X: lx, Y: 0, Z: lz, Block: e.To}
	}

	for key, m := range patches {
		st := c.voxelChunks[key]
		if st == nil || st.needsFull {
			continue
		}
		cells := make([]observerproto.ChunkVoxelPatchCell, 0, len(m))
		for _, cell := range m {
			cells = append(cells, cell)
		}
		sort.Slice(cells, func(i, j int) bool {
			if cells[i].Y != cells[j].Y {
				return cells[i].Y < cells[j].Y
			}
			if cells[i].Z != cells[j].Z {
				return cells[i].Z < cells[j].Z
			}
			return cells[i].X < cells[j].X
		})
		msg := observerproto.ChunkVoxelPatchMsg{
			Type:            "CHUNK_VOXEL_PATCH",
			ProtocolVersion: observerproto.Version,
			CX:              key.CX,
			CZ:              key.CZ,
			Cells:           cells,
		}
		b, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		if !trySend(c.dataOut, b) {
			// Force a full resync next tick if PATCH is dropped.
			st.needsFull = true
		}
	}

	// Evictions.
	if len(c.voxelChunks) > 0 {
		var evictKeys []ChunkKey
		for k, st := range c.voxelChunks {
			if _, ok := wantSet[k]; ok {
				continue
			}
			if nowTick-st.lastWantedTick < observerVoxelEvictAfterTicks {
				continue
			}
			if !st.sentFull {
				evictKeys = append(evictKeys, k)
				continue
			}

			msg := observerproto.ChunkVoxelsEvictMsg{
				Type:            "CHUNK_VOXELS_EVICT",
				ProtocolVersion: observerproto.Version,
				CX:              k.CX,
				CZ:              k.CZ,
			}
			b, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			if trySend(c.dataOut, b) {
				evictKeys = append(evictKeys, k)
			}
		}
		for _, k := range evictKeys {
			delete(c.voxelChunks, k)
		}
	}
}

func (w *World) sendChunkSurface(c *observerClient, st *observerChunk) bool {
	if w == nil || c == nil || st == nil || st.surface == nil {
		return false
	}
	msg := observerproto.ChunkSurfaceMsg{
		Type:            "CHUNK_SURFACE",
		ProtocolVersion: observerproto.Version,
		CX:              st.key.CX,
		CZ:              st.key.CZ,
		Encoding:        "PAL16_Y8",
		Data:            encodePAL16Y8(st.surface),
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	return trySend(c.dataOut, b)
}

func (w *World) sendChunkVoxels(c *observerClient, st *observerVoxelChunk) bool {
	if w == nil || c == nil || st == nil || st.blocks == nil {
		return false
	}
	msg := observerproto.ChunkVoxelsMsg{
		Type:            "CHUNK_VOXELS",
		ProtocolVersion: observerproto.Version,
		CX:              st.key.CX,
		CZ:              st.key.CZ,
		Encoding:        "PAL16_U16LE_YZX",
		Data:            encodePAL16U16LE(st.blocks),
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	return trySend(c.dataOut, b)
}

func encodePAL16Y8(surface []surfaceCell) string {
	buf := make([]byte, 16*16*3)
	for i, c := range surface {
		off := i * 3
		buf[off] = byte(c.b)
		buf[off+1] = byte(c.b >> 8)
		buf[off+2] = c.y
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func encodePAL16U16LE(blocks []uint16) string {
	buf := make([]byte, len(blocks)*2)
	for i, v := range blocks {
		off := i * 2
		buf[off] = byte(v)
		buf[off+1] = byte(v >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func (w *World) computeChunkSurface(cx, cz int) []surfaceCell {
	ch := w.chunkForSurface(cx, cz)
	air := w.chunks.gen.Air
	boundaryR := w.chunks.gen.BoundaryR
	out := make([]surfaceCell, 16*16)
	for z := 0; z < 16; z++ {
		for x := 0; x < 16; x++ {
			wx := cx*16 + x
			wz := cz*16 + z
			if boundaryR > 0 && (wx < -boundaryR || wx > boundaryR || wz < -boundaryR || wz > boundaryR) {
				out[z*16+x] = surfaceCell{b: air, y: 0}
				continue
			}
			b := air
			v := ch.Blocks[x+z*16]
			if v != air {
				b = v
			}
			out[z*16+x] = surfaceCell{b: b, y: 0}
		}
	}
	return out
}

func (w *World) computeChunkVoxels(cx, cz int) []uint16 {
	ch := w.chunkForVoxels(cx, cz)
	if ch == nil || ch.Blocks == nil {
		return nil
	}
	out := make([]uint16, len(ch.Blocks))
	copy(out, ch.Blocks)

	// Respect world boundary the same way ChunkStore.GetBlock does.
	if w.chunks != nil && w.chunks.gen.BoundaryR > 0 {
		air := w.chunks.gen.Air
		br := w.chunks.gen.BoundaryR
		for z := 0; z < 16; z++ {
			for x := 0; x < 16; x++ {
				wx := cx*16 + x
				wz := cz*16 + z
				if wx < -br || wx > br || wz < -br || wz > br {
					out[x+z*16] = air
				}
			}
		}
	}
	return out
}

func (w *World) computeSurfaceCellAt(wx, wz int) surfaceCell {
	air := w.chunks.gen.Air
	boundaryR := w.chunks.gen.BoundaryR
	if boundaryR > 0 && (wx < -boundaryR || wx > boundaryR || wz < -boundaryR || wz > boundaryR) {
		return surfaceCell{b: air, y: 0}
	}
	cx := floorDiv(wx, 16)
	cz := floorDiv(wz, 16)
	lx := mod(wx, 16)
	lz := mod(wz, 16)
	ch := w.chunkForSurface(cx, cz)
	v := ch.Blocks[lx+lz*16]
	if v != air {
		return surfaceCell{b: v, y: 0}
	}
	return surfaceCell{b: air, y: 0}
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

func computeWantedChunks(agents []ChunkKey, radius int, maxChunks int) []ChunkKey {
	if radius <= 0 {
		radius = 1
	}
	if maxChunks <= 0 {
		maxChunks = 1024
	}
	type item struct {
		k    ChunkKey
		dist int
	}
	distByKey := map[ChunkKey]int{}
	for _, a := range agents {
		for dz := -radius; dz <= radius; dz++ {
			for dx := -radius; dx <= radius; dx++ {
				k := ChunkKey{CX: a.CX + dx, CZ: a.CZ + dz}
				d := abs(dx) + abs(dz)
				if prev, ok := distByKey[k]; !ok || d < prev {
					distByKey[k] = d
				}
			}
		}
	}
	items := make([]item, 0, len(distByKey))
	for k, d := range distByKey {
		items = append(items, item{k: k, dist: d})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].dist != items[j].dist {
			return items[i].dist < items[j].dist
		}
		if items[i].k.CX != items[j].k.CX {
			return items[i].k.CX < items[j].k.CX
		}
		return items[i].k.CZ < items[j].k.CZ
	})
	if len(items) > maxChunks {
		items = items[:maxChunks]
	}
	out := make([]ChunkKey, 0, len(items))
	for _, it := range items {
		out = append(out, it.k)
	}
	return out
}

func trySend(ch chan []byte, b []byte) bool {
	select {
	case ch <- b:
		return true
	default:
		return false
	}
}

func clampInt(v, min, max, def int) int {
	if v == 0 {
		v = def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ceil is a tiny helper to avoid importing math in the world loop hot path.
func ceil(v float64) float64 {
	i := int(v)
	if v == float64(i) {
		return v
	}
	if v > 0 {
		return float64(i + 1)
	}
	return float64(i)
}
