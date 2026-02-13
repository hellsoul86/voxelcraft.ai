package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"voxelcraft.ai/internal/openclaw/bridge"
	"voxelcraft.ai/internal/openclaw/mcp"
)

func main() {
	var (
		listen     = flag.String("listen", "127.0.0.1:8090", "http listen address")
		worldWSURL = flag.String("world-ws-url", "ws://127.0.0.1:8080/v1/ws", "voxelcraft ws url")
		hmacSecret = flag.String("hmac-secret", "", "hmac secret (or set VC_MCP_HMAC_SECRET)")
		stateFile  = flag.String("state-file", "./data/mcp/sessions.json", "path to persisted session state (resume tokens)")
		maxSess    = flag.Int("max-sessions", 256, "max concurrent sessions")
	)
	flag.Parse()

	if strings.TrimSpace(*hmacSecret) == "" {
		*hmacSecret = strings.TrimSpace(os.Getenv("VC_MCP_HMAC_SECRET"))
	}
	requireHMAC := envBoolWithDefault("VC_MCP_REQUIRE_HMAC", defaultRequireMCPHMAC())
	allowLegacyHMAC := envBoolWithDefault("VC_MCP_HMAC_ALLOW_LEGACY", defaultAllowLegacyHMAC())
	if requireHMAC && strings.TrimSpace(*hmacSecret) == "" {
		log.Fatalf("[mcp] hmac secret required (set -hmac-secret or VC_MCP_HMAC_SECRET)")
	}
	if strings.TrimSpace(*hmacSecret) == "" && !isLoopbackListenAddress(*listen) {
		log.Fatalf("[mcp] refusing insecure MCP bind on non-loopback address %q without hmac secret", *listen)
	}

	logger := log.New(os.Stdout, "[mcp] ", log.LstdFlags|log.Lmicroseconds)
	authMode := "none"
	if strings.TrimSpace(*hmacSecret) == "" {
		authMode = "none(loopback-only)"
	} else {
		authMode = "hmac"
	}
	logger.Printf("auth_mode=%s require_hmac=%t allow_legacy_hmac=%t", authMode, requireHMAC, allowLegacyHMAC)

	br, err := bridge.NewManager(bridge.Config{
		WorldWSURL:  *worldWSURL,
		StateFile:   *stateFile,
		MaxSessions: *maxSess,
	})
	if err != nil {
		logger.Fatalf("bridge: %v", err)
	}
	defer br.Close()

	srv, err := mcp.NewServer(mcp.Config{
		Bridge:     br,
		HMACSecret: *hmacSecret,
	})
	if err != nil {
		logger.Fatalf("mcp: %v", err)
	}

	httpSrv := &http.Server{
		Addr:              *listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, cancel := signalContext()
	defer cancel()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	logger.Printf("listening on http://%s (world ws=%s)", *listen, *worldWSURL)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("listen: %v", err)
	}
}

func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()
	return ctx, cancel
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

func envBoolWithDefault(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
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

