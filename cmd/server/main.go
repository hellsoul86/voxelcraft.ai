package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	persistlog "voxelcraft.ai/internal/persistence/log"
	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/world"
	"voxelcraft.ai/internal/transport/ws"
)

func main() {
	var (
		addr      = flag.String("addr", ":8080", "http listen address")
		worldID   = flag.String("world", "world_1", "world id")
		seed      = flag.Int64("seed", 1337, "world seed (used only when starting a fresh world)")
		configDir = flag.String("configs", "./configs", "config directory")
		dataDir   = flag.String("data", "./data", "runtime data directory")

		snapPath   = flag.String("snapshot", "", "path to snapshot to load (optional)")
		loadLatest = flag.Bool("load_latest_snapshot", true, "load latest snapshot from data dir if present (when -snapshot is empty)")
	)
	flag.Parse()

	logger := log.New(os.Stdout, "[server] ", log.LstdFlags|log.Lmicroseconds)

	cats, err := catalogs.Load(*configDir)
	if err != nil {
		logger.Fatalf("load catalogs: %v", err)
	}

	worldDir := filepath.Join(*dataDir, "worlds", *worldID)
	_ = os.MkdirAll(worldDir, 0o755)

	// Create world (fresh or resumed from snapshot).
	var w *world.World
	snapshotToLoad := strings.TrimSpace(*snapPath)
	if snapshotToLoad == "" && *loadLatest {
		snapshotToLoad = latestSnapshot(worldDir)
	}
	if snapshotToLoad != "" {
		snap, err := snapshot.ReadSnapshot(snapshotToLoad)
		if err != nil {
			logger.Fatalf("read snapshot: %v", err)
		}
		if snap.Header.WorldID != "" && snap.Header.WorldID != *worldID {
			logger.Fatalf("snapshot world id mismatch: flag=%s snap=%s", *worldID, snap.Header.WorldID)
		}
		w, err = world.New(world.WorldConfig{
			ID:         *worldID,
			TickRateHz: snap.TickRate,
			DayTicks:   snap.DayTicks,
			ObsRadius:  snap.ObsRadius,
			Height:     snap.Height,
			Seed:       snap.Seed,
			BoundaryR:  snap.BoundaryR,
		}, cats)
		if err != nil {
			logger.Fatalf("world: %v", err)
		}
		if err := w.ImportSnapshot(snap); err != nil {
			logger.Fatalf("import snapshot: %v", err)
		}
		logger.Printf("resumed from snapshot=%s tick=%d", filepath.Base(snapshotToLoad), w.CurrentTick())
	} else {
		w, err = world.New(world.WorldConfig{
			ID:         *worldID,
			TickRateHz: 5,
			DayTicks:   6000,
			ObsRadius:  7,
			Height:     64,
			Seed:       *seed,
			BoundaryR:  4000,
		}, cats)
		if err != nil {
			logger.Fatalf("world: %v", err)
		}
	}

	ctx, cancel := signalContext()
	defer cancel()

	tickLog := persistlog.NewTickLogger(worldDir)
	auditLog := persistlog.NewAuditLogger(worldDir)
	defer tickLog.Close()
	defer auditLog.Close()
	w.SetTickLogger(tickLog)
	w.SetAuditLogger(auditLog)

	// Snapshot writer.
	snapCh := make(chan snapshot.SnapshotV1, 2)
	w.SetSnapshotSink(snapCh)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case snap := <-snapCh:
				path := filepath.Join(worldDir, "snapshots", fmt.Sprintf("%d.snap.zst", snap.Header.Tick))
				if err := snapshot.WriteSnapshot(path, snap); err != nil {
					logger.Printf("snapshot write: %v", err)
				}
			}
		}
	}()

	go func() {
		if err := w.Run(ctx); err != nil && err != context.Canceled {
			logger.Printf("world stopped: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(200)
		_, _ = rw.Write([]byte("ok"))
	})
	mux.HandleFunc("/metrics", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "text/plain; version=0.0.4")
		// Minimal Prometheus exposition format.
		fmt.Fprintf(rw, "# HELP voxelcraft_world_tick Current world tick.\n")
		fmt.Fprintf(rw, "# TYPE voxelcraft_world_tick gauge\n")
		fmt.Fprintf(rw, "voxelcraft_world_tick{world=%q} %d\n", *worldID, w.CurrentTick())
	})
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.HandleFunc("/v1/ws", ws.NewServer(w, logger).Handler())

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		_ = srv.Shutdown(ctx2)
	}()

	logger.Printf("listening on %s", *addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("ListenAndServe: %v", err)
	}
}

func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()
	return ctx, cancel
}

func latestSnapshot(worldDir string) string {
	dir := filepath.Join(worldDir, "snapshots")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var best string
	var bestTick uint64
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".snap.zst") {
			continue
		}
		base := strings.TrimSuffix(name, ".snap.zst")
		tick, err := strconv.ParseUint(base, 10, 64)
		if err != nil {
			continue
		}
		if best == "" || tick > bestTick {
			bestTick = tick
			best = filepath.Join(dir, name)
		}
	}
	return best
}
