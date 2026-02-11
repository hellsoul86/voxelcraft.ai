package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"voxelcraft.ai/internal/openclaw/bridge"
)

type stubBridge struct{}

func (stubBridge) GetStatus(ctx context.Context, sessionKey string) (bridge.Status, error) {
	_ = ctx
	_ = sessionKey
	return bridge.Status{Connected: false, WorldWSURL: "ws://example.invalid/v1/ws"}, nil
}
func (stubBridge) ListWorlds(ctx context.Context, sessionKey string) ([]bridge.WorldInfo, error) {
	_ = ctx
	_ = sessionKey
	return []bridge.WorldInfo{{WorldID: "OVERWORLD", WorldType: "OVERWORLD"}}, nil
}
func (stubBridge) GetObs(ctx context.Context, sessionKey string, opts bridge.GetObsOpts) (bridge.ObsResult, error) {
	_ = ctx
	_ = sessionKey
	_ = opts
	return bridge.ObsResult{Tick: 0, AgentID: "A1", Obs: nil}, nil
}
func (stubBridge) GetCatalog(ctx context.Context, sessionKey, name string) (bridge.CatalogResult, error) {
	_ = ctx
	_ = sessionKey
	_ = name
	return bridge.CatalogResult{Name: "block_palette", Digest: "d", Data: json.RawMessage(`["AIR"]`)}, nil
}
func (stubBridge) Act(ctx context.Context, sessionKey string, args bridge.ActArgs) (bridge.ActResult, error) {
	_ = ctx
	_ = sessionKey
	_ = args
	return bridge.ActResult{Sent: true, TickUsed: 1, AgentID: "A1"}, nil
}
func (stubBridge) Disconnect(ctx context.Context, sessionKey string) error {
	_ = ctx
	_ = sessionKey
	return nil
}

func rpcPost(t *testing.T, base string, payload any, headers map[string]string) rpcResponse {
	t.Helper()
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", base+"/mcp", bytes.NewReader(b))
	req.Header.Set("content-type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
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

func TestMCP_Initialize_And_ListTools(t *testing.T) {
	s, err := NewServer(Config{Bridge: stubBridge{}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	initResp := rpcPost(t, ts.URL, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	}, nil)
	if initResp.Error != nil {
		t.Fatalf("initialize error: %+v", initResp.Error)
	}
	rm, _ := initResp.Result.(map[string]any)
	if rm["protocolVersion"] == "" {
		t.Fatalf("missing protocolVersion in result")
	}

	lt := rpcPost(t, ts.URL, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "list_tools",
	}, nil)
	if lt.Error != nil {
		t.Fatalf("list_tools error: %+v", lt.Error)
	}
	rm2, ok := lt.Result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected list_tools result type: %T", lt.Result)
	}
	tools, ok := rm2["tools"].([]any)
	if !ok {
		t.Fatalf("missing tools array")
	}
	if len(tools) != 6 {
		t.Fatalf("expected 6 tools, got %d", len(tools))
	}
}

func TestMCP_CallTool_Unknown(t *testing.T) {
	s, err := NewServer(Config{Bridge: stubBridge{}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp := rpcPost(t, ts.URL, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "call_tool",
		"params": map[string]any{
			"name":      "nope",
			"arguments": map[string]any{},
		},
	}, nil)
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("expected tool not found (-32601), got %+v", resp.Error)
	}
}
