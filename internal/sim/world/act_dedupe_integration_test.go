package world

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %s", dir)
		}
		dir = parent
	}
}

func TestWorldActDedupe_CheckOrRemember(t *testing.T) {
	root := findRepoRoot(t)
	cats, err := catalogs.Load(filepath.Join(root, "configs"))
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := New(WorldConfig{
		ID:         "W1",
		TickRateHz: 20,
		Height:     1,
		ObsRadius:  7,
		Seed:       42,
	}, cats)
	if err != nil {
		t.Fatalf("new world: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = w.Run(ctx) }()

	callCtx, callCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer callCancel()
	first := protocol.AckMsg{
		Type:            protocol.TypeAck,
		ProtocolVersion: "1.1",
		AckFor:          "ACT_1",
		Accepted:        true,
		ServerTick:      1,
		WorldID:         "W1",
	}
	got1, dup1, err := w.RequestCheckOrRememberActAck(callCtx, "A1", "W1", "ACT_1", first)
	if err != nil {
		t.Fatalf("first dedupe call: %v", err)
	}
	if dup1 {
		t.Fatalf("first call should not be duplicate")
	}
	if got1.AckFor != "ACT_1" {
		t.Fatalf("unexpected first ack: %+v", got1)
	}

	other := first
	other.Message = "should_be_ignored"
	got2, dup2, err := w.RequestCheckOrRememberActAck(callCtx, "A1", "W1", "ACT_1", other)
	if err != nil {
		t.Fatalf("second dedupe call: %v", err)
	}
	if !dup2 {
		t.Fatalf("second call should be duplicate")
	}
	if got2.Message != first.Message {
		t.Fatalf("duplicate should return original ack, got message=%q want %q", got2.Message, first.Message)
	}
}
