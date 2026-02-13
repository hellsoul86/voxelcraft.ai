package runtime

import (
	configpkg "voxelcraft.ai/internal/sim/world/feature/observer/config"
	streamspkg "voxelcraft.ai/internal/sim/world/feature/observer/stream"
)

type JoinRequest struct {
	SessionID string
	TickOut   chan []byte
	DataOut   chan []byte

	ChunkRadius int
	MaxChunks   int

	FocusAgentID   string
	VoxelRadius    int
	VoxelMaxChunks int
}

type SubscribeRequest struct {
	SessionID string

	ChunkRadius int
	MaxChunks   int

	FocusAgentID   string
	VoxelRadius    int
	VoxelMaxChunks int
}

func JoinSession(observers map[string]*streamspkg.Client, req JoinRequest) {
	if observers == nil || req.SessionID == "" || req.TickOut == nil || req.DataOut == nil {
		return
	}
	cfgIn := configpkg.NewObserverCfg(req.ChunkRadius, req.MaxChunks, req.FocusAgentID, req.VoxelRadius, req.VoxelMaxChunks)

	if old := observers[req.SessionID]; old != nil {
		close(old.TickOut)
		close(old.DataOut)
	}
	observers[req.SessionID] = &streamspkg.Client{
		ID:      req.SessionID,
		TickOut: req.TickOut,
		DataOut: req.DataOut,
		Config: streamspkg.Config{
			ChunkRadius:    cfgIn.ChunkRadius,
			MaxChunks:      cfgIn.MaxChunks,
			FocusAgentID:   cfgIn.FocusAgentID,
			VoxelRadius:    cfgIn.VoxelRadius,
			VoxelMaxChunks: cfgIn.VoxelMaxChunks,
		},
		Chunks:      map[streamspkg.ChunkKey]*streamspkg.ChunkState{},
		VoxelChunks: map[streamspkg.ChunkKey]*streamspkg.VoxelState{},
	}
}

func SubscribeSession(observers map[string]*streamspkg.Client, req SubscribeRequest) {
	if observers == nil || req.SessionID == "" {
		return
	}
	c := observers[req.SessionID]
	if c == nil {
		return
	}
	next := configpkg.ApplySubscription(configpkg.ObserverCfg{
		ChunkRadius:    c.Config.ChunkRadius,
		MaxChunks:      c.Config.MaxChunks,
		FocusAgentID:   c.Config.FocusAgentID,
		VoxelRadius:    c.Config.VoxelRadius,
		VoxelMaxChunks: c.Config.VoxelMaxChunks,
	}, req.ChunkRadius, req.MaxChunks, req.FocusAgentID, req.VoxelRadius, req.VoxelMaxChunks)
	c.Config.ChunkRadius = next.ChunkRadius
	c.Config.MaxChunks = next.MaxChunks
	c.Config.FocusAgentID = next.FocusAgentID
	c.Config.VoxelRadius = next.VoxelRadius
	c.Config.VoxelMaxChunks = next.VoxelMaxChunks
}

func LeaveSession(observers map[string]*streamspkg.Client, sessionID string) {
	if observers == nil || sessionID == "" {
		return
	}
	c := observers[sessionID]
	if c == nil {
		return
	}
	delete(observers, sessionID)
	close(c.TickOut)
	close(c.DataOut)
}
