package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"voxelcraft.ai/internal/openclaw/bridge"
	"voxelcraft.ai/internal/openclaw/mcp"
)

func main() {
	var (
		listen     = flag.String("listen", "127.0.0.1:8090", "http listen address")
		worldWSURL = flag.String("world-ws-url", "ws://127.0.0.1:8080/v1/ws", "voxelcraft ws url")
		hmacSecret = flag.String("hmac-secret", "", "optional hmac secret; when set, requires x-agent-id/x-ts/x-signature")
		stateFile  = flag.String("state-file", "./data/mcp/sessions.json", "path to persisted session state (resume tokens)")
		maxSess    = flag.Int("max-sessions", 256, "max concurrent sessions")
	)
	flag.Parse()

	logger := log.New(os.Stdout, "[mcp] ", log.LstdFlags|log.Lmicroseconds)

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

