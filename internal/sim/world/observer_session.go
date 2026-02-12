package world

import (
	boardspkg "voxelcraft.ai/internal/sim/world/feature/observer/boards"
	configpkg "voxelcraft.ai/internal/sim/world/feature/observer/config"
)

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

func (w *World) newPostID() string {
	return boardspkg.NewPostID(w.nextPostNum.Add(1))
}

func boardIDAt(pos Vec3i) string {
	return containerID("BULLETIN_BOARD", pos)
}

func (w *World) ensureBoard(pos Vec3i) *Board {
	id := boardIDAt(pos)
	b := w.boards[id]
	if b != nil {
		return b
	}
	b = &Board{BoardID: id}
	w.boards[id] = b
	return b
}

func (w *World) removeBoard(pos Vec3i) {
	delete(w.boards, boardIDAt(pos))
}

func (w *World) handleObserverJoin(req ObserverJoinRequest) {
	if w == nil || req.SessionID == "" || req.TickOut == nil || req.DataOut == nil {
		return
	}

	cfg := observerCfg{
		chunkRadius:    clampInt(configpkg.ClampChunkRadius(req.ChunkRadius), 1, 32, 6),
		maxChunks:      clampInt(configpkg.ClampMaxChunks(req.MaxChunks), 1, 16384, 1024),
		focusAgentID:   configpkg.NormalizeFocusAgentID(req.FocusAgentID),
		voxelRadius:    clampInt(configpkg.ClampVoxelRadius(req.VoxelRadius), 0, 8, 0),
		voxelMaxChunks: clampInt(configpkg.ClampVoxelMaxChunks(req.VoxelMaxChunks), 1, 2048, 256),
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
	c.cfg.focusAgentID = configpkg.NormalizeFocusAgentID(req.FocusAgentID)
	c.cfg.voxelRadius = configpkg.ClampVoxelRadius(req.VoxelRadius)
	if req.VoxelMaxChunks > 0 {
		c.cfg.voxelMaxChunks = clampInt(configpkg.ClampVoxelMaxChunks(req.VoxelMaxChunks), 1, 2048, c.cfg.voxelMaxChunks)
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
