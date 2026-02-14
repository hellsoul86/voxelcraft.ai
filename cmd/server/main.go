package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"voxelcraft.ai/internal/persistence/archive"
	persistlog "voxelcraft.ai/internal/persistence/log"
	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/multiworld"
	"voxelcraft.ai/internal/sim/tuning"
	"voxelcraft.ai/internal/sim/world"
	"voxelcraft.ai/internal/transport/observer"
	"voxelcraft.ai/internal/transport/ws"
)

func main() {
	var (
		addr       = flag.String("addr", ":8080", "http listen address")
		worldID    = flag.String("world", "world_1", "world id")
		seed       = flag.Int64("seed", 1337, "world seed (used only when starting a fresh world)")
		configDir  = flag.String("configs", "./configs", "config directory")
		worldsPath = flag.String("worlds", "./configs/worlds.yaml", "multi-world config path (if exists, server runs in multi-world mode)")
		dataDir    = flag.String("data", "./data", "runtime data directory")
		tuningPath = flag.String("tuning", "", "path to tuning.yaml (default: <configs>/tuning.yaml)")
		disableDB  = flag.Bool("disable_db", false, "disable indexing (tick/audit + catalogs + snapshot metadata)")

		snapPath   = flag.String("snapshot", "", "path to snapshot to load (optional)")
		loadLatest = flag.Bool("load_latest_snapshot", true, "load latest snapshot from data dir if present (when -snapshot is empty)")

		mcpListen     = flag.String("mcp_listen", "127.0.0.1:8090", "embedded MCP http listen address (empty to disable)")
		mcpWorldWSURL = flag.String("mcp_world_ws_url", "", "world ws url for embedded MCP (default: ws://127.0.0.1:<server-port>/v1/ws)")
		mcpHMACSecret = flag.String("mcp_hmac_secret", "", "embedded MCP hmac secret (or set VC_MCP_HMAC_SECRET)")
		mcpStateFile  = flag.String("mcp_state_file", "", "embedded MCP persisted session state file (default: <data>/mcp/sessions.json)")
		mcpMaxSessions = flag.Int("mcp_max_sessions", 256, "embedded MCP max concurrent sessions")
	)
	flag.Parse()

	logger := log.New(os.Stdout, "[server] ", log.LstdFlags|log.Lmicroseconds)

	mcpStatePath := strings.TrimSpace(*mcpStateFile)
	if mcpStatePath == "" {
		mcpStatePath = filepath.Join(*dataDir, "mcp", "sessions.json")
	}
	mcpCfg := embeddedMCPCfg{
		Listen:        strings.TrimSpace(*mcpListen),
		WorldHTTPAddr: strings.TrimSpace(*addr),
		WorldWSURL:    strings.TrimSpace(*mcpWorldWSURL),
		StateFile:     mcpStatePath,
		MaxSessions:   *mcpMaxSessions,
		HMACSecret:    strings.TrimSpace(*mcpHMACSecret),
	}

	cats, err := catalogs.Load(*configDir)
	if err != nil {
		logger.Fatalf("load catalogs: %v", err)
	}

	worldDir := filepath.Join(*dataDir, "worlds", *worldID)
	_ = os.MkdirAll(worldDir, 0o755)

	tp := strings.TrimSpace(*tuningPath)
	if tp == "" {
		tp = filepath.Join(*configDir, "tuning.yaml")
	}

	// Optional: read-model index backend (does not affect sim determinism).
	idx, err := openRuntimeIndex(worldDir, *worldID, *disableDB, logger)
	if err != nil {
		logger.Fatalf("open index backend: %v", err)
	}
	if idx != nil {
		defer idx.Close()
	}

	// Create world (fresh or resumed from snapshot).
	var w *world.World
	snapshotToLoad := strings.TrimSpace(*snapPath)
	if snapshotToLoad == "" && *loadLatest {
		snapshotToLoad = latestSnapshot(worldDir)
	}

	// Load tuning (required for fresh world; optional for snapshot resumes).
	tune, tuneErr := tuning.Load(tp)
	if tuneErr != nil {
		if snapshotToLoad == "" {
			logger.Fatalf("load tuning: %v", tuneErr)
		}
		// Resume fallback: snapshot should contain the effective tuning; allow missing file.
		if os.IsNotExist(tuneErr) {
			logger.Printf("tuning not found (%s); using defaults", tp)
			tune = tuning.Defaults()
		} else {
			logger.Fatalf("load tuning: %v", tuneErr)
		}
	}

	// Multi-world 1.0 mode (if worlds config exists). Keep single-world flow as fallback.
	if strings.TrimSpace(*worldsPath) != "" {
		if _, err := os.Stat(*worldsPath); err == nil {
			mcfg, err := multiworld.Load(*worldsPath)
			if err != nil {
				logger.Fatalf("load worlds config: %v", err)
			}
			runMultiWorld(serverRuntimeConfig{
				Addr:      *addr,
				DataDir:   *dataDir,
				DisableDB: *disableDB,
				Seed:      *seed,
				ConfigDir: *configDir,
			}, mcfg, tune, cats, mcpCfg, logger)
			return
		}
	}

	r2Mirror, err := buildR2MirrorRuntime(*dataDir, logger)
	if err != nil {
		logger.Fatalf("init r2 mirror: %v", err)
	}
	defer r2Mirror.Close()

	if idx != nil {
		if err := idx.UpsertCatalogs(*configDir, cats, tune); err != nil {
			logger.Printf("index backend: upsert catalogs: %v", err)
		}
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
			ID:                              *worldID,
			TickRateHz:                      snap.TickRate,
			DayTicks:                        snap.DayTicks,
			SeasonLengthTicks:               tune.SeasonLengthTicks,
			ObsRadius:                       snap.ObsRadius,
			Height:                          snap.Height,
			Seed:                            snap.Seed,
			BoundaryR:                       snap.BoundaryR,
			BiomeRegionSize:                 tune.WorldGen.BiomeRegionSize,
			SpawnClearRadius:                tune.WorldGen.SpawnClearRadius,
			OreClusterProbScalePermille:     tune.WorldGen.OreClusterProbScalePermille,
			TerrainClusterProbScalePermille: tune.WorldGen.TerrainClusterProbScalePermille,
			SprinkleStonePermille:           tune.WorldGen.SprinkleStonePermille,
			SprinkleDirtPermille:            tune.WorldGen.SprinkleDirtPermille,
			SprinkleLogPermille:             tune.WorldGen.SprinkleLogPermille,
			StarterItems:                    tune.StarterItems,
			SnapshotEveryTicks:              tune.SnapshotEveryTicks,
			DirectorEveryTicks:              tune.DirectorEveryTicks,
			RateLimits: world.RateLimitConfig{
				SayWindowTicks:        tune.RateLimits.SayWindowTicks,
				SayMax:                tune.RateLimits.SayMax,
				MarketSayWindowTicks:  tune.RateLimits.MarketSayWindowTicks,
				MarketSayMax:          tune.RateLimits.MarketSayMax,
				WhisperWindowTicks:    tune.RateLimits.WhisperWindowTicks,
				WhisperMax:            tune.RateLimits.WhisperMax,
				OfferTradeWindowTicks: tune.RateLimits.OfferTradeWindowTicks,
				OfferTradeMax:         tune.RateLimits.OfferTradeMax,
				PostBoardWindowTicks:  tune.RateLimits.PostBoardWindowTicks,
				PostBoardMax:          tune.RateLimits.PostBoardMax,
			},
			LawNoticeTicks:         tune.LawNoticeTicks,
			LawVoteTicks:           tune.LawVoteTicks,
			BlueprintAutoPullRange: tune.BlueprintAutoPullRange,
			BlueprintBlocksPerTick: tune.BlueprintBlocksPerTick,
			AccessPassCoreRadius:   tune.AccessPassCoreRadius,
			MaintenanceCost:        tune.ClaimMaintenanceCost,
			FunDecayWindowTicks:    tune.FunDecayWindowTicks,
			FunDecayBase:           tune.FunDecayBase,
			StructureSurvivalTicks: tune.StructureSurvivalTicks,
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
			ID:                              *worldID,
			TickRateHz:                      tune.TickRateHz,
			DayTicks:                        tune.DayTicks,
			SeasonLengthTicks:               tune.SeasonLengthTicks,
			ObsRadius:                       tune.ObsRadius,
			Height:                          tune.ChunkSize[2],
			Seed:                            *seed,
			BoundaryR:                       tune.WorldBoundaryR,
			BiomeRegionSize:                 tune.WorldGen.BiomeRegionSize,
			SpawnClearRadius:                tune.WorldGen.SpawnClearRadius,
			OreClusterProbScalePermille:     tune.WorldGen.OreClusterProbScalePermille,
			TerrainClusterProbScalePermille: tune.WorldGen.TerrainClusterProbScalePermille,
			SprinkleStonePermille:           tune.WorldGen.SprinkleStonePermille,
			SprinkleDirtPermille:            tune.WorldGen.SprinkleDirtPermille,
			SprinkleLogPermille:             tune.WorldGen.SprinkleLogPermille,
			StarterItems:                    tune.StarterItems,
			SnapshotEveryTicks:              tune.SnapshotEveryTicks,
			DirectorEveryTicks:              tune.DirectorEveryTicks,
			RateLimits: world.RateLimitConfig{
				SayWindowTicks:        tune.RateLimits.SayWindowTicks,
				SayMax:                tune.RateLimits.SayMax,
				MarketSayWindowTicks:  tune.RateLimits.MarketSayWindowTicks,
				MarketSayMax:          tune.RateLimits.MarketSayMax,
				WhisperWindowTicks:    tune.RateLimits.WhisperWindowTicks,
				WhisperMax:            tune.RateLimits.WhisperMax,
				OfferTradeWindowTicks: tune.RateLimits.OfferTradeWindowTicks,
				OfferTradeMax:         tune.RateLimits.OfferTradeMax,
				PostBoardWindowTicks:  tune.RateLimits.PostBoardWindowTicks,
				PostBoardMax:          tune.RateLimits.PostBoardMax,
			},
			LawNoticeTicks:         tune.LawNoticeTicks,
			LawVoteTicks:           tune.LawVoteTicks,
			BlueprintAutoPullRange: tune.BlueprintAutoPullRange,
			BlueprintBlocksPerTick: tune.BlueprintBlocksPerTick,
			AccessPassCoreRadius:   tune.AccessPassCoreRadius,
			MaintenanceCost:        tune.ClaimMaintenanceCost,
			FunDecayWindowTicks:    tune.FunDecayWindowTicks,
			FunDecayBase:           tune.FunDecayBase,
			StructureSurvivalTicks: tune.StructureSurvivalTicks,
		}, cats)
		if err != nil {
			logger.Fatalf("world: %v", err)
		}
	}

	ctx, cancel := signalContext()
	defer cancel()

	embeddedMCP, err := startEmbeddedMCP(ctx, mcpCfg, logger)
	if err != nil {
		logger.Fatalf("embedded mcp: %v", err)
	}
	defer func() {
		if embeddedMCP != nil {
			embeddedMCP.Close()
		}
	}()

	logOpts := persistlog.LoggerOptions{}
	if r2Mirror != nil && r2Mirror.enabled {
		logOpts.RotateLayout = r2Mirror.rotateLayout
		logOpts.OnClose = r2Mirror.Enqueue
	}
	tickLog := persistlog.NewTickLoggerWithOptions(worldDir, logOpts)
	auditLog := persistlog.NewAuditLoggerWithOptions(worldDir, logOpts)
	defer tickLog.Close()
	defer auditLog.Close()
	w.SetTickLogger(multiTickLogger{a: tickLog, b: idx})
	w.SetAuditLogger(multiAuditLogger{a: auditLog, b: idx})

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
					continue
				}

				if r2Mirror != nil && r2Mirror.enabled {
					r2Mirror.Enqueue(path)
				}

				if idx != nil {
					idx.RecordSnapshot(path, snap)
					idx.RecordSnapshotState(snap)
					if season, archivedPath, ok, err := archive.ArchiveSeasonSnapshot(worldDir, path, snap); err != nil {
						logger.Printf("archive season snapshot: %v", err)
					} else if ok {
						idx.RecordSeason(season, snap.Header.Tick, archivedPath, snap.Seed)
						if r2Mirror != nil && r2Mirror.enabled {
							r2Mirror.Enqueue(archivedPath)
							enqueueIfExists(r2Mirror, filepath.Join(filepath.Dir(archivedPath), "meta.json"))
						}
					}
					continue
				}

				// Archive even when index db is disabled.
				if _, archivedPath, ok, err := archive.ArchiveSeasonSnapshot(worldDir, path, snap); err != nil {
					logger.Printf("archive season snapshot: %v", err)
				} else if ok && r2Mirror != nil && r2Mirror.enabled {
					r2Mirror.Enqueue(archivedPath)
					enqueueIfExists(r2Mirror, filepath.Join(filepath.Dir(archivedPath), "meta.json"))
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

		m := w.Metrics()
		tick := w.CurrentTick()
		if m.Tick != 0 {
			tick = m.Tick
		}

		// Minimal Prometheus exposition format.
		fmt.Fprintf(rw, "# HELP voxelcraft_world_tick Current world tick.\n")
		fmt.Fprintf(rw, "# TYPE voxelcraft_world_tick gauge\n")
		fmt.Fprintf(rw, "voxelcraft_world_tick{world=%q} %d\n", *worldID, tick)

		fmt.Fprintf(rw, "# HELP voxelcraft_world_agents Current number of agents in the world.\n")
		fmt.Fprintf(rw, "# TYPE voxelcraft_world_agents gauge\n")
		fmt.Fprintf(rw, "voxelcraft_world_agents{world=%q} %d\n", *worldID, m.Agents)

		fmt.Fprintf(rw, "# HELP voxelcraft_world_clients Current number of connected clients.\n")
		fmt.Fprintf(rw, "# TYPE voxelcraft_world_clients gauge\n")
		fmt.Fprintf(rw, "voxelcraft_world_clients{world=%q} %d\n", *worldID, m.Clients)

		fmt.Fprintf(rw, "# HELP voxelcraft_world_loaded_chunks Loaded chunk count.\n")
		fmt.Fprintf(rw, "# TYPE voxelcraft_world_loaded_chunks gauge\n")
		fmt.Fprintf(rw, "voxelcraft_world_loaded_chunks{world=%q} %d\n", *worldID, m.LoadedChunks)

		fmt.Fprintf(rw, "# HELP voxelcraft_world_queue_depth Channel backlog depth.\n")
		fmt.Fprintf(rw, "# TYPE voxelcraft_world_queue_depth gauge\n")
		fmt.Fprintf(rw, "voxelcraft_world_queue_depth{world=%q,queue=%q} %d\n", *worldID, "inbox", m.QueueDepths.Inbox)
		fmt.Fprintf(rw, "voxelcraft_world_queue_depth{world=%q,queue=%q} %d\n", *worldID, "join", m.QueueDepths.Join)
		fmt.Fprintf(rw, "voxelcraft_world_queue_depth{world=%q,queue=%q} %d\n", *worldID, "leave", m.QueueDepths.Leave)
		fmt.Fprintf(rw, "voxelcraft_world_queue_depth{world=%q,queue=%q} %d\n", *worldID, "attach", m.QueueDepths.Attach)

		fmt.Fprintf(rw, "# HELP voxelcraft_world_step_ms Last tick step duration in milliseconds.\n")
		fmt.Fprintf(rw, "# TYPE voxelcraft_world_step_ms gauge\n")
		fmt.Fprintf(rw, "voxelcraft_world_step_ms{world=%q} %.3f\n", *worldID, m.StepMS)

		fmt.Fprintf(rw, "# HELP voxelcraft_director_metric Director metrics (0..1).\n")
		fmt.Fprintf(rw, "# TYPE voxelcraft_director_metric gauge\n")
		fmt.Fprintf(rw, "voxelcraft_director_metric{world=%q,metric=%q} %.6f\n", *worldID, "trade", m.Director.Trade)
		fmt.Fprintf(rw, "voxelcraft_director_metric{world=%q,metric=%q} %.6f\n", *worldID, "conflict", m.Director.Conflict)
		fmt.Fprintf(rw, "voxelcraft_director_metric{world=%q,metric=%q} %.6f\n", *worldID, "exploration", m.Director.Exploration)
		fmt.Fprintf(rw, "voxelcraft_director_metric{world=%q,metric=%q} %.6f\n", *worldID, "inequality", m.Director.Inequality)
		fmt.Fprintf(rw, "voxelcraft_director_metric{world=%q,metric=%q} %.6f\n", *worldID, "public_infra", m.Director.PublicInfra)

		fmt.Fprintf(rw, "# HELP voxelcraft_stats_window Rolling window stats.\n")
		fmt.Fprintf(rw, "# TYPE voxelcraft_stats_window gauge\n")
		fmt.Fprintf(rw, "voxelcraft_stats_window{world=%q,metric=%q} %d\n", *worldID, "trades", m.StatsWindow.Trades)
		fmt.Fprintf(rw, "voxelcraft_stats_window{world=%q,metric=%q} %d\n", *worldID, "denied", m.StatsWindow.Denied)
		fmt.Fprintf(rw, "voxelcraft_stats_window{world=%q,metric=%q} %d\n", *worldID, "chunks_discovered", m.StatsWindow.ChunksDiscovered)
		fmt.Fprintf(rw, "voxelcraft_stats_window{world=%q,metric=%q} %d\n", *worldID, "blueprints_complete", m.StatsWindow.BlueprintsComplete)

		fmt.Fprintf(rw, "# HELP voxelcraft_stats_window_ticks Rolling window size in ticks.\n")
		fmt.Fprintf(rw, "# TYPE voxelcraft_stats_window_ticks gauge\n")
		fmt.Fprintf(rw, "voxelcraft_stats_window_ticks{world=%q} %d\n", *worldID, m.StatsWindowTicks)

		writeR2MirrorMetrics(rw, r2Mirror)
	})

	enableAdminHTTP := envBool("VC_ENABLE_ADMIN_HTTP", defaultEnableAdminHTTP())
	enablePprofHTTP := envBool("VC_ENABLE_PPROF_HTTP", false)
	if enableAdminHTTP {
		// Local-only admin endpoints (do not affect simulation determinism).
		mux.HandleFunc("/admin/v1/state", func(rw http.ResponseWriter, r *http.Request) {
			if !isLoopbackRemote(r.RemoteAddr) {
				http.Error(rw, "forbidden", http.StatusForbidden)
				return
			}
			rw.Header().Set("Content-Type", "application/json")
			resp := struct {
				WorldID string             `json:"world_id"`
				Tick    uint64             `json:"tick"`
				Metrics world.WorldMetrics `json:"metrics"`
			}{
				WorldID: *worldID,
				Tick:    w.CurrentTick(),
				Metrics: w.Metrics(),
			}
			_ = json.NewEncoder(rw).Encode(resp)
		})
		mux.HandleFunc("/admin/v1/snapshot", func(rw http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				rw.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemote(r.RemoteAddr) {
				http.Error(rw, "forbidden", http.StatusForbidden)
				return
			}
			ctx2, cancel2 := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel2()
			tick, err := w.RequestSnapshot(ctx2)
			rw.Header().Set("Content-Type", "application/json")
			if err != nil {
				rw.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(rw).Encode(map[string]any{"ok": false, "tick": tick, "error": err.Error()})
				return
			}
			_ = json.NewEncoder(rw).Encode(map[string]any{"ok": true, "tick": tick})
		})

		obsSrv := observer.NewServer(w, logger)
		mux.HandleFunc("/admin/v1/observer/bootstrap", obsSrv.BootstrapHandler())
		mux.HandleFunc("/admin/v1/observer/ws", obsSrv.WSHandler())
	} else {
		logger.Printf("admin endpoints disabled (VC_ENABLE_ADMIN_HTTP=false)")
	}
	if enablePprofHTTP {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	} else {
		logger.Printf("pprof endpoints disabled (VC_ENABLE_PPROF_HTTP=false)")
	}
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

func isLoopbackRemote(remoteAddr string) bool {
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func enqueueIfExists(m *r2MirrorRuntime, path string) {
	if m == nil || !m.enabled {
		return
	}
	if _, err := os.Stat(path); err == nil {
		m.Enqueue(path)
	}
}

func defaultEnableAdminHTTP() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DEPLOY_ENV"))) {
	case "staging", "production":
		return false
	default:
		return true
	}
}

func writeR2MirrorMetrics(rw http.ResponseWriter, mirror *r2MirrorRuntime) {
	if mirror == nil || !mirror.enabled {
		return
	}
	s := mirror.Stats()
	if !s.Enabled {
		return
	}
	fmt.Fprintf(rw, "# HELP voxelcraft_r2_mirror_queue_depth Current R2 mirror queue depth.\n")
	fmt.Fprintf(rw, "# TYPE voxelcraft_r2_mirror_queue_depth gauge\n")
	fmt.Fprintf(rw, "voxelcraft_r2_mirror_queue_depth %d\n", s.QueueDepth)

	fmt.Fprintf(rw, "# HELP voxelcraft_r2_mirror_queue_capacity R2 mirror queue capacity.\n")
	fmt.Fprintf(rw, "# TYPE voxelcraft_r2_mirror_queue_capacity gauge\n")
	fmt.Fprintf(rw, "voxelcraft_r2_mirror_queue_capacity %d\n", s.QueueCapacity)

	fmt.Fprintf(rw, "# HELP voxelcraft_r2_mirror_enqueued_total Total mirror enqueue attempts.\n")
	fmt.Fprintf(rw, "# TYPE voxelcraft_r2_mirror_enqueued_total counter\n")
	fmt.Fprintf(rw, "voxelcraft_r2_mirror_enqueued_total %d\n", s.EnqueuedTotal)

	fmt.Fprintf(rw, "# HELP voxelcraft_r2_mirror_queue_saturated_total Total enqueue attempts when queue was saturated.\n")
	fmt.Fprintf(rw, "# TYPE voxelcraft_r2_mirror_queue_saturated_total counter\n")
	fmt.Fprintf(rw, "voxelcraft_r2_mirror_queue_saturated_total %d\n", s.QueueSaturatedTotal)

	fmt.Fprintf(rw, "# HELP voxelcraft_r2_mirror_dropped_total Total mirror files dropped because queue remained saturated.\n")
	fmt.Fprintf(rw, "# TYPE voxelcraft_r2_mirror_dropped_total counter\n")
	fmt.Fprintf(rw, "voxelcraft_r2_mirror_dropped_total %d\n", s.DroppedTotal)

	fmt.Fprintf(rw, "# HELP voxelcraft_r2_mirror_upload_success_total Total successful mirror uploads.\n")
	fmt.Fprintf(rw, "# TYPE voxelcraft_r2_mirror_upload_success_total counter\n")
	fmt.Fprintf(rw, "voxelcraft_r2_mirror_upload_success_total %d\n", s.UploadSuccessTotal)

	fmt.Fprintf(rw, "# HELP voxelcraft_r2_mirror_upload_fail_total Total failed mirror uploads after retry.\n")
	fmt.Fprintf(rw, "# TYPE voxelcraft_r2_mirror_upload_fail_total counter\n")
	fmt.Fprintf(rw, "voxelcraft_r2_mirror_upload_fail_total %d\n", s.UploadFailTotal)

	fmt.Fprintf(rw, "# HELP voxelcraft_r2_mirror_last_success_unix Unix timestamp of last successful mirror upload.\n")
	fmt.Fprintf(rw, "# TYPE voxelcraft_r2_mirror_last_success_unix gauge\n")
	fmt.Fprintf(rw, "voxelcraft_r2_mirror_last_success_unix %d\n", s.LastSuccessUnix)

	fmt.Fprintf(rw, "# HELP voxelcraft_r2_mirror_last_error_unix Unix timestamp of last failed mirror upload.\n")
	fmt.Fprintf(rw, "# TYPE voxelcraft_r2_mirror_last_error_unix gauge\n")
	fmt.Fprintf(rw, "voxelcraft_r2_mirror_last_error_unix %d\n", s.LastErrorUnix)
}

type multiTickLogger struct {
	a world.TickLogger
	b world.TickLogger
}

func (m multiTickLogger) WriteTick(entry world.TickLogEntry) error {
	if m.a != nil {
		_ = m.a.WriteTick(entry)
	}
	if m.b != nil {
		_ = m.b.WriteTick(entry)
	}
	return nil
}

type multiAuditLogger struct {
	a world.AuditLogger
	b world.AuditLogger
}

func (m multiAuditLogger) WriteAudit(entry world.AuditEntry) error {
	if m.a != nil {
		_ = m.a.WriteAudit(entry)
	}
	if m.b != nil {
		_ = m.b.WriteAudit(entry)
	}
	return nil
}
