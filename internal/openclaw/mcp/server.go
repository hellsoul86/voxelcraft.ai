package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"voxelcraft.ai/internal/openclaw/bridge"
)

type Bridge interface {
	GetStatus(ctx context.Context, sessionKey string) (bridge.Status, error)
	GetObs(ctx context.Context, sessionKey string, opts bridge.GetObsOpts) (bridge.ObsResult, error)
	GetCatalog(ctx context.Context, sessionKey, name string) (bridge.CatalogResult, error)
	Act(ctx context.Context, sessionKey string, args bridge.ActArgs) (bridge.ActResult, error)
	Disconnect(ctx context.Context, sessionKey string) error
}

type Config struct {
	Bridge     Bridge
	HMACSecret string
}

type Server struct {
	bridge     Bridge
	hmacSecret []byte
	now        func() time.Time
}

func NewServer(cfg Config) (*Server, error) {
	if cfg.Bridge == nil {
		return nil, fmt.Errorf("nil bridge")
	}
	s := &Server{
		bridge: cfg.Bridge,
		now:    time.Now,
	}
	if strings.TrimSpace(cfg.HMACSecret) != "" {
		s.hmacSecret = []byte(cfg.HMACSecret)
	}
	return s, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(200)
		_, _ = rw.Write([]byte("ok"))
	})
	mux.HandleFunc("/mcp", s.handleMCP)
	return mux
}

func (s *Server) handleMCP(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		_, _ = rw.Write([]byte("bad body"))
		return
	}
	_ = r.Body.Close()

	// Optional HMAC auth.
	sessionKey := strings.TrimSpace(r.Header.Get(headerAgentID))
	if len(s.hmacSecret) > 0 {
		vr := verifyHMAC(r, body, s.hmacSecret, s.now())
		if vr.HTTPStatus != 0 {
			rw.WriteHeader(vr.HTTPStatus)
			_, _ = rw.Write([]byte(vr.Message))
			return
		}
		sessionKey = vr.SessionKey
	}
	if sessionKey == "" {
		sessionKey = "default"
	}

	req, err := parseRPCRequest(body)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		_, _ = rw.Write([]byte("bad jsonrpc request"))
		return
	}

	resp := s.dispatch(r.Context(), sessionKey, req)
	rw.Header().Set("content-type", "application/json")
	enc := json.NewEncoder(rw)
	_ = enc.Encode(resp)
}

func (s *Server) dispatch(ctx context.Context, sessionKey string, req rpcRequest) rpcResponse {
	switch req.Method {
	case "initialize":
		return rpcOK(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
		})

	case "list_tools":
		return rpcOK(req.ID, map[string]any{"tools": s.toolsList()})

	case "call_tool":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if len(req.Params) == 0 {
			return rpcErr(req.ID, -32602, "missing params", nil)
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return rpcErr(req.ID, -32602, "bad params", err.Error())
		}
		if p.Name == "" {
			return rpcErr(req.ID, -32602, "missing tool name", nil)
		}
		if !isKnownTool(p.Name) {
			return rpcErr(req.ID, -32601, "tool not found", map[string]any{"name": p.Name})
		}
		out, err := s.callTool(ctx, sessionKey, p.Name, p.Arguments)
		if err != nil {
			return rpcErr(req.ID, -32000, err.Error(), nil)
		}
		return rpcOK(req.ID, out)

	default:
		return rpcErr(req.ID, -32601, "method not found", nil)
	}
}

func (s *Server) toolsList() []map[string]any {
	// Minimal MCP-ish tool descriptors. OpenClaw only needs name + arguments shape.
	return []map[string]any{
		{
			"name":        "voxelcraft.get_status",
			"description": "Get current session status for the backing VoxelCraft WS connection.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
		},
		{
			"name":        "voxelcraft.get_obs",
			"description": "Get the latest OBS (optionally wait for a new tick).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"mode": map[string]any{"type": "string", "enum": []string{"full", "no_voxels", "summary"}},
					"wait_new_tick": map[string]any{"type": "boolean"},
					"timeout_ms": map[string]any{"type": "integer"},
				},
			},
		},
		{
			"name":        "voxelcraft.get_catalog",
			"description": "Get a catalog (block_palette/item_palette/tuning/recipes/blueprints/law_templates/events).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
				"required": []string{"name"},
			},
		},
		{
			"name":        "voxelcraft.act",
			"description": "Send an ACT to the VoxelCraft server. Tick/protocol_version/agent_id are auto-filled from latest OBS.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"instants": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
					"tasks": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
					"cancel": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
			},
		},
		{
			"name":        "voxelcraft.disconnect",
			"description": "Disconnect the backing VoxelCraft WS session (resume token is kept).",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
		},
	}
}

func (s *Server) callTool(ctx context.Context, sessionKey string, name string, args json.RawMessage) (any, error) {
	switch name {
	case "voxelcraft.get_status":
		st, err := s.bridge.GetStatus(ctx, sessionKey)
		if err != nil {
			return nil, err
		}
		return st, nil

	case "voxelcraft.get_obs":
		var o bridge.GetObsOpts
		if len(args) > 0 {
			if err := json.Unmarshal(args, &o); err != nil {
				return nil, fmt.Errorf("bad arguments: %w", err)
			}
		}
		res, err := s.bridge.GetObs(ctx, sessionKey, o)
		if err != nil {
			return nil, err
		}
		return res, nil

	case "voxelcraft.get_catalog":
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("bad arguments: %w", err)
		}
		if p.Name == "" {
			return nil, fmt.Errorf("missing name")
		}
		res, err := s.bridge.GetCatalog(ctx, sessionKey, p.Name)
		if err != nil {
			return nil, err
		}
		return res, nil

	case "voxelcraft.act":
		var a bridge.ActArgs
		if len(args) > 0 {
			if err := json.Unmarshal(args, &a); err != nil {
				return nil, fmt.Errorf("bad arguments: %w", err)
			}
		}
		res, err := s.bridge.Act(ctx, sessionKey, a)
		if err != nil {
			return nil, err
		}
		return res, nil

	case "voxelcraft.disconnect":
		if err := s.bridge.Disconnect(ctx, sessionKey); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func isKnownTool(name string) bool {
	switch name {
	case "voxelcraft.get_status",
		"voxelcraft.get_obs",
		"voxelcraft.get_catalog",
		"voxelcraft.act",
		"voxelcraft.disconnect":
		return true
	default:
		return false
	}
}
