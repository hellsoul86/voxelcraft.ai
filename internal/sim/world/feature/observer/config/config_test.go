package config

import "testing"

func TestNewObserverCfgDefaultsAndClamp(t *testing.T) {
	cfg := NewObserverCfg(0, 0, "  A1  ", -1, 0)
	if cfg.ChunkRadius != 1 {
		t.Fatalf("expected clamped chunk radius 1, got %d", cfg.ChunkRadius)
	}
	if cfg.MaxChunks != 1 {
		t.Fatalf("expected clamped max chunks 1, got %d", cfg.MaxChunks)
	}
	if cfg.FocusAgentID != "A1" {
		t.Fatalf("expected trimmed focus id, got %q", cfg.FocusAgentID)
	}
	if cfg.VoxelRadius != 0 {
		t.Fatalf("expected voxel radius 0, got %d", cfg.VoxelRadius)
	}
	if cfg.VoxelMaxChunks != 1 {
		t.Fatalf("expected clamped voxel max chunks 1, got %d", cfg.VoxelMaxChunks)
	}
}

func TestApplySubscription(t *testing.T) {
	base := ObserverCfg{
		ChunkRadius:    6,
		MaxChunks:      1024,
		FocusAgentID:   "A1",
		VoxelRadius:    2,
		VoxelMaxChunks: 256,
	}
	next := ApplySubscription(base, 8, 2048, "B2", 3, 300)
	if next.ChunkRadius != 8 || next.MaxChunks != 2048 {
		t.Fatalf("expected chunk/max update, got %+v", next)
	}
	if next.FocusAgentID != "B2" {
		t.Fatalf("expected focus id B2, got %q", next.FocusAgentID)
	}
	if next.VoxelRadius != 3 || next.VoxelMaxChunks != 300 {
		t.Fatalf("expected voxel updates, got %+v", next)
	}
}
