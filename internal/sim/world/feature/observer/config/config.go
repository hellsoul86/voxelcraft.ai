package config

import "strings"

type ObserverCfg struct {
	ChunkRadius int
	MaxChunks   int

	FocusAgentID   string
	VoxelRadius    int
	VoxelMaxChunks int
}

func ClampChunkRadius(v int) int {
	if v < 1 {
		return 1
	}
	if v > 32 {
		return 32
	}
	return v
}

func ClampMaxChunks(v int) int {
	if v < 1 {
		return 1
	}
	if v > 16384 {
		return 16384
	}
	return v
}

func ClampVoxelRadius(v int) int {
	if v < 0 {
		return 0
	}
	if v > 8 {
		return 8
	}
	return v
}

func ClampVoxelMaxChunks(v int) int {
	if v < 1 {
		return 1
	}
	if v > 2048 {
		return 2048
	}
	return v
}

func NormalizeFocusAgentID(v string) string {
	return strings.TrimSpace(v)
}

func NewObserverCfg(chunkRadius, maxChunks int, focusAgentID string, voxelRadius, voxelMaxChunks int) ObserverCfg {
	return ObserverCfg{
		ChunkRadius:    clampWithDefault(ClampChunkRadius(chunkRadius), 1, 32, 6),
		MaxChunks:      clampWithDefault(ClampMaxChunks(maxChunks), 1, 16384, 1024),
		FocusAgentID:   NormalizeFocusAgentID(focusAgentID),
		VoxelRadius:    clampWithDefault(ClampVoxelRadius(voxelRadius), 0, 8, 0),
		VoxelMaxChunks: clampWithDefault(ClampVoxelMaxChunks(voxelMaxChunks), 1, 2048, 256),
	}
}

func ApplySubscription(cfg ObserverCfg, chunkRadius, maxChunks int, focusAgentID string, voxelRadius, voxelMaxChunks int) ObserverCfg {
	out := cfg
	out.ChunkRadius = clampWithDefault(chunkRadius, 1, 32, cfg.ChunkRadius)
	out.MaxChunks = clampWithDefault(maxChunks, 1, 16384, cfg.MaxChunks)
	out.FocusAgentID = NormalizeFocusAgentID(focusAgentID)
	out.VoxelRadius = ClampVoxelRadius(voxelRadius)
	if voxelMaxChunks > 0 {
		out.VoxelMaxChunks = clampWithDefault(ClampVoxelMaxChunks(voxelMaxChunks), 1, 2048, cfg.VoxelMaxChunks)
	}
	return out
}

func clampWithDefault(v, min, max, def int) int {
	if v < min || v > max {
		return def
	}
	return v
}
