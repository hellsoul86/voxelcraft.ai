package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
func (stubBridge) GetEvents(ctx context.Context, sessionKey string, sinceCursor uint64, limit int) (bridge.GetEventsResult, error) {
	_ = ctx
	_ = sessionKey
	_ = sinceCursor
	_ = limit
	return bridge.GetEventsResult{Events: nil, NextCursor: 0}, nil
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

type readyzBridge struct {
	statusCalls int
}

func (b *readyzBridge) GetStatus(ctx context.Context, sessionKey string) (bridge.Status, error) {
	_ = ctx
	_ = sessionKey
	b.statusCalls++
	if b.statusCalls == 1 {
		return bridge.Status{Connected: false, LastObsTick: 0, WorldWSURL: "ws://example.invalid/v1/ws"}, nil
	}
	return bridge.Status{Connected: true, LastObsTick: 9, WorldWSURL: "ws://example.invalid/v1/ws"}, nil
}

func (b *readyzBridge) ListWorlds(ctx context.Context, sessionKey string) ([]bridge.WorldInfo, error) {
	_ = ctx
	_ = sessionKey
	return []bridge.WorldInfo{{WorldID: "OVERWORLD", WorldType: "OVERWORLD"}}, nil
}

func (b *readyzBridge) GetObs(ctx context.Context, sessionKey string, opts bridge.GetObsOpts) (bridge.ObsResult, error) {
	_ = ctx
	_ = sessionKey
	_ = opts
	return bridge.ObsResult{Tick: 9, AgentID: "readyz", Obs: json.RawMessage(`{"tick":9}`)}, nil
}

func (b *readyzBridge) GetEvents(ctx context.Context, sessionKey string, sinceCursor uint64, limit int) (bridge.GetEventsResult, error) {
	_ = ctx
	_ = sessionKey
	_ = sinceCursor
	_ = limit
	return bridge.GetEventsResult{}, nil
}

func (b *readyzBridge) GetCatalog(ctx context.Context, sessionKey, name string) (bridge.CatalogResult, error) {
	_ = ctx
	_ = sessionKey
	_ = name
	return bridge.CatalogResult{}, nil
}

func (b *readyzBridge) Act(ctx context.Context, sessionKey string, args bridge.ActArgs) (bridge.ActResult, error) {
	_ = ctx
	_ = sessionKey
	_ = args
	return bridge.ActResult{}, nil
}

func (b *readyzBridge) Disconnect(ctx context.Context, sessionKey string) error {
	_ = ctx
	_ = sessionKey
	return nil
}

type readyzFailBridge struct{}

func (readyzFailBridge) GetStatus(ctx context.Context, sessionKey string) (bridge.Status, error) {
	_ = ctx
	_ = sessionKey
	return bridge.Status{Connected: false, LastObsTick: 0, WorldWSURL: "ws://example.invalid/v1/ws"}, nil
}
func (readyzFailBridge) ListWorlds(ctx context.Context, sessionKey string) ([]bridge.WorldInfo, error) {
	_ = ctx
	_ = sessionKey
	return nil, nil
}
func (readyzFailBridge) GetObs(ctx context.Context, sessionKey string, opts bridge.GetObsOpts) (bridge.ObsResult, error) {
	_ = ctx
	_ = sessionKey
	_ = opts
	return bridge.ObsResult{}, errors.New("ws disconnected")
}
func (readyzFailBridge) GetEvents(ctx context.Context, sessionKey string, sinceCursor uint64, limit int) (bridge.GetEventsResult, error) {
	_ = ctx
	_ = sessionKey
	_ = sinceCursor
	_ = limit
	return bridge.GetEventsResult{}, nil
}
func (readyzFailBridge) GetCatalog(ctx context.Context, sessionKey, name string) (bridge.CatalogResult, error) {
	_ = ctx
	_ = sessionKey
	_ = name
	return bridge.CatalogResult{}, nil
}
func (readyzFailBridge) Act(ctx context.Context, sessionKey string, args bridge.ActArgs) (bridge.ActResult, error) {
	_ = ctx
	_ = sessionKey
	_ = args
	return bridge.ActResult{}, nil
}
func (readyzFailBridge) Disconnect(ctx context.Context, sessionKey string) error {
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

func postRaw(t *testing.T, url string, payload []byte, headers map[string]string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", url, bytes.NewReader(payload))
	req.Header.Set("content-type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post raw: %v", err)
	}
	return res
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
	if len(tools) != 7 {
		t.Fatalf("expected 7 tools, got %d", len(tools))
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

func TestMCP_Readyz_WarmsBridge(t *testing.T) {
	b := &readyzBridge{}
	s, err := NewServer(Config{Bridge: b})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatalf("readyz get: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
}

func TestMCP_Readyz_Unhealthy(t *testing.T) {
	s, err := NewServer(Config{Bridge: readyzFailBridge{}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatalf("readyz get: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", res.StatusCode)
	}
}

func TestMCP_HMACReplayRejected(t *testing.T) {
	s, err := NewServer(Config{Bridge: stubBridge{}, HMACSecret: "topsecret"})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"list_tools"}`)
	tsStr := "1700000000000"
	nonce := "nonce_replay"
	sig := signHMAC([]byte("topsecret"), canonicalStringV2(tsStr, "POST", "/mcp", "agent_1", nonce, payload))
	headers := map[string]string{
		headerAgentID:   "agent_1",
		headerTS:        tsStr,
		headerNonce:     nonce,
		headerSignature: sig,
	}

	baseNow := time.UnixMilli(1700000000000)
	s.now = func() time.Time { return baseNow }

	res1 := postRaw(t, ts.URL+"/mcp", payload, headers)
	defer res1.Body.Close()
	if res1.StatusCode != http.StatusOK {
		t.Fatalf("expected first call 200, got %d", res1.StatusCode)
	}

	res2 := postRaw(t, ts.URL+"/mcp", payload, headers)
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusConflict {
		t.Fatalf("expected replay call 409, got %d", res2.StatusCode)
	}
}
