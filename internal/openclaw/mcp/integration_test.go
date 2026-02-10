package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"voxelcraft.ai/internal/openclaw/bridge"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/world"
	"voxelcraft.ai/internal/transport/ws"
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

func TestMCP_Sidecar_EndToEnd_WS(t *testing.T) {
	root := findRepoRoot(t)
	cats, err := catalogs.Load(filepath.Join(root, "configs"))
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	w, err := world.New(world.WorldConfig{
		ID:         "test_world",
		TickRateHz: 50,
		ObsRadius:  7,
		Height:     1,
		Seed:       123,
	}, cats)
	if err != nil {
		t.Fatalf("world: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = w.Run(ctx) }()

	// World WS server.
	wsLogger := log.New(io.Discard, "", 0)
	wsSrv := ws.NewServer(w, wsLogger)
	muxWorld := http.NewServeMux()
	muxWorld.HandleFunc("/v1/ws", wsSrv.Handler())
	tsWorld := httptest.NewServer(muxWorld)
	defer tsWorld.Close()

	worldWSURL := "ws" + strings.TrimPrefix(tsWorld.URL, "http") + "/v1/ws"

	// MCP sidecar.
	stateDir := t.TempDir()
	br, err := bridge.NewManager(bridge.Config{
		WorldWSURL:  worldWSURL,
		StateFile:   filepath.Join(stateDir, "sessions.json"),
		MaxSessions: 16,
	})
	if err != nil {
		t.Fatalf("bridge: %v", err)
	}
	defer br.Close()

	mcpSrv, err := NewServer(Config{Bridge: br})
	if err != nil {
		t.Fatalf("mcp: %v", err)
	}
	tsMCP := httptest.NewServer(mcpSrv.Handler())
	defer tsMCP.Close()

	// Helper: JSON-RPC call_tool.
	callTool := func(t *testing.T, id int, name string, args any) rpcResponse {
		t.Helper()
		payload := map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  "call_tool",
			"params": map[string]any{
				"name":      name,
				"arguments": args,
			},
		}
		b, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", tsMCP.URL+"/mcp", bytes.NewReader(b))
		req.Header.Set("content-type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		defer res.Body.Close()
		var out rpcResponse
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return out
	}

	// Wait for an OBS tick.
	obsResp := callTool(t, 1, "voxelcraft.get_obs", map[string]any{
		"mode":          "summary",
		"wait_new_tick": true,
		"timeout_ms":    2000,
	})
	if obsResp.Error != nil {
		t.Fatalf("get_obs error: %+v", obsResp.Error)
	}
	obsRes, ok := obsResp.Result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected get_obs result: %T", obsResp.Result)
	}
	tickF, _ := obsRes["tick"].(float64)
	if tickF <= 0 {
		t.Fatalf("expected tick > 0, got %v", obsRes["tick"])
	}
	agentID, _ := obsRes["agent_id"].(string)
	if agentID == "" {
		t.Fatalf("expected agent_id")
	}

	// Send an ACT (SAY).
	actResp := callTool(t, 2, "voxelcraft.act", map[string]any{
		"instants": []map[string]any{
			{"id": "I1", "type": "SAY", "channel": "LOCAL", "text": "hello"},
		},
	})
	if actResp.Error != nil {
		t.Fatalf("act error: %+v", actResp.Error)
	}

	// Next OBS should contain ACTION_RESULT for I1.
	obs2 := callTool(t, 3, "voxelcraft.get_obs", map[string]any{
		"mode":          "summary",
		"wait_new_tick": true,
		"timeout_ms":    2000,
	})
	if obs2.Error != nil {
		t.Fatalf("get_obs2 error: %+v", obs2.Error)
	}
	obs2Res, ok := obs2.Result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected get_obs2 result: %T", obs2.Result)
	}
	obsObj, ok := obs2Res["obs"].(map[string]any)
	if !ok {
		t.Fatalf("expected obs object")
	}
	events, _ := obsObj["events"].([]any)
	found := false
	for _, e := range events {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if m["type"] == "ACTION_RESULT" && m["ref"] == "I1" {
			if okv, _ := m["ok"].(bool); okv {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("expected ACTION_RESULT for ref=I1 in events; agent_id=%s", agentID)
	}

	// Cleanup.
	cancel()
	// Let goroutines observe cancel.
	time.Sleep(50 * time.Millisecond)
}
