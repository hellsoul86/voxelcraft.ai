package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"voxelcraft.ai/internal/openclaw/bridge"
	"voxelcraft.ai/internal/openclaw/mcp"
)

type embeddedMCPCfg struct {
	// Listen is the HTTP listen address for the embedded MCP server.
	// Set to empty to disable.
	Listen string

	// WorldHTTPAddr is the main voxelcraft HTTP listen addr (used to derive ws://.../v1/ws).
	WorldHTTPAddr string

	// WorldWSURL overrides the derived ws://.../v1/ws endpoint.
	WorldWSURL string

	// StateFile persists per-session resume tokens for the MCP bridge.
	StateFile string

	// MaxSessions is the max number of concurrent MCP sessions the bridge keeps.
	MaxSessions int

	// HMACSecret overrides VC_MCP_HMAC_SECRET.
	HMACSecret string
}

type embeddedMCP struct {
	httpSrv *http.Server
	ln      net.Listener
	bridge  *bridge.Manager

	closeOnce sync.Once
}

func (e *embeddedMCP) Close() {
	if e == nil {
		return
	}
	e.closeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if e.httpSrv != nil {
			_ = e.httpSrv.Shutdown(ctx)
		}
		if e.ln != nil {
			_ = e.ln.Close()
		}
		if e.bridge != nil {
			_ = e.bridge.Close()
		}
	})
}

func startEmbeddedMCP(ctx context.Context, cfg embeddedMCPCfg, logger *log.Logger) (*embeddedMCP, error) {
	listen := strings.TrimSpace(cfg.Listen)
	if listen == "" {
		if logger != nil {
			logger.Printf("embedded MCP disabled (mcp_listen empty)")
		}
		return nil, nil
	}
	if logger == nil {
		logger = log.New(os.Stdout, "[mcp] ", log.LstdFlags|log.Lmicroseconds)
	}

	worldWSURL := strings.TrimSpace(cfg.WorldWSURL)
	if worldWSURL == "" {
		derived, err := worldWSURLFromHTTPAddr(cfg.WorldHTTPAddr)
		if err != nil {
			return nil, err
		}
		worldWSURL = derived
	}

	stateFile := strings.TrimSpace(cfg.StateFile)
	if stateFile == "" {
		stateFile = "./data/mcp/sessions.json"
	}
	if dir := filepath.Dir(stateFile); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}

	secret := strings.TrimSpace(cfg.HMACSecret)
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("VC_MCP_HMAC_SECRET"))
	}

	requireHMAC := envBool("VC_MCP_REQUIRE_HMAC", defaultRequireMCPHMAC())
	allowLegacyHMAC := envBool("VC_MCP_HMAC_ALLOW_LEGACY", defaultAllowLegacyHMAC())

	if requireHMAC && secret == "" {
		return nil, fmt.Errorf("[mcp] hmac secret required (set -mcp_hmac_secret or VC_MCP_HMAC_SECRET)")
	}
	if secret == "" && !isLoopbackListenAddress(listen) {
		return nil, fmt.Errorf("[mcp] refusing insecure MCP listen on non-loopback address %q without hmac secret", listen)
	}

	authMode := "none(loopback-only)"
	if secret != "" {
		authMode = "hmac"
	}

	logger.Printf("embedded_mcp auth_mode=%s require_hmac=%t allow_legacy_hmac=%t", authMode, requireHMAC, allowLegacyHMAC)
	logger.Printf("embedded_mcp listening on http://%s (world ws=%s)", listen, worldWSURL)

	maxSess := cfg.MaxSessions
	if maxSess <= 0 {
		maxSess = 256
	}
	br, err := bridge.NewManager(bridge.Config{WorldWSURL: worldWSURL, StateFile: stateFile, MaxSessions: maxSess})
	if err != nil {
		return nil, fmt.Errorf("mcp bridge: %w", err)
	}

	mcpSrv, err := mcp.NewServer(mcp.Config{Bridge: br, HMACSecret: secret})
	if err != nil {
		_ = br.Close()
		return nil, fmt.Errorf("mcp server: %w", err)
	}

	ln, err := net.Listen("tcp", listen)
	if err != nil {
		_ = br.Close()
		return nil, fmt.Errorf("mcp listen: %w", err)
	}

	httpSrv := &http.Server{
		Addr:              listen,
		Handler:           mcpSrv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	em := &embeddedMCP{httpSrv: httpSrv, ln: ln, bridge: br}

	go func() {
		<-ctx.Done()
		em.Close()
	}()

	go func() {
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Printf("embedded_mcp serve error: %v", err)
		}
	}()

	return em, nil
}

func defaultRequireMCPHMAC() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DEPLOY_ENV"))) {
	case "staging", "production":
		return true
	default:
		return false
	}
}

func defaultAllowLegacyHMAC() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DEPLOY_ENV"))) {
	case "staging", "production":
		return false
	default:
		return true
	}
}

func isLoopbackListenAddress(addr string) bool {
	host := strings.TrimSpace(addr)
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = strings.TrimSpace(h)
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		return false
	}
	hostLower := strings.ToLower(host)
	if hostLower == "localhost" {
		return true
	}
	ip := net.ParseIP(hostLower)
	return ip != nil && ip.IsLoopback()
}

func worldWSURLFromHTTPAddr(httpAddr string) (string, error) {
	addr := strings.TrimSpace(httpAddr)
	if addr == "" {
		return "", fmt.Errorf("empty http addr")
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// Fallback: allow bare port ("8080")
		if n, aErr := strconv.Atoi(addr); aErr == nil && n > 0 {
			host = "127.0.0.1"
			port = addr
			err = nil
		}
	}
	if err != nil {
		return "", fmt.Errorf("bad http addr %q: %w", httpAddr, err)
	}
	port = strings.TrimSpace(port)
	if port == "" {
		return "", fmt.Errorf("missing port in http addr %q", httpAddr)
	}

	h := strings.Trim(strings.TrimSpace(host), "[]")
	if h == "" || h == "0.0.0.0" || h == "::" || strings.EqualFold(h, "localhost") {
		h = "127.0.0.1"
	}
	// If h is IPv6, bracket it.
	if strings.Contains(h, ":") {
		h = "[" + h + "]"
	}

	return fmt.Sprintf("ws://%s:%s/v1/ws", h, port), nil
}
