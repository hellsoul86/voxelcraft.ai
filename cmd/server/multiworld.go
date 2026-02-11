package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"voxelcraft.ai/internal/persistence/archive"
	"voxelcraft.ai/internal/persistence/indexdb"
	persistlog "voxelcraft.ai/internal/persistence/log"
	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/multiworld"
	"voxelcraft.ai/internal/sim/tuning"
	"voxelcraft.ai/internal/sim/world"
	"voxelcraft.ai/internal/transport/ws"
)

type serverRuntimeConfig struct {
	Addr      string
	DataDir   string
	DisableDB bool
	Seed      int64
	ConfigDir string
}

func runMultiWorld(rtCfg serverRuntimeConfig, cfg multiworld.Config, tune tuning.Tuning, cats *catalogs.Catalogs, logger *log.Logger) {
	ctx, cancel := signalContext()
	defer cancel()

	runtimes := map[string]*multiworld.Runtime{}

	for _, spec := range cfg.Worlds {
		worldDir := filepath.Join(rtCfg.DataDir, "worlds", spec.ID)
		_ = os.MkdirAll(worldDir, 0o755)

		var idx *indexdb.SQLiteIndex
		if !rtCfg.DisableDB {
			dbPath := filepath.Join(worldDir, "index", "world.sqlite")
			var err error
			idx, err = indexdb.OpenSQLite(dbPath)
			if err != nil {
				logger.Fatalf("open index db (%s): %v", spec.ID, err)
			}
			defer idx.Close()
			if err := idx.UpsertCatalogs(rtCfg.ConfigDir, cats, tune); err != nil {
				logger.Printf("index db upsert catalogs (%s): %v", spec.ID, err)
			}
		}

		w, err := world.New(world.WorldConfig{
			ID:                              spec.ID,
			WorldType:                       spec.Type,
			TickRateHz:                      tune.TickRateHz,
			DayTicks:                        tune.DayTicks,
			SeasonLengthTicks:               tune.SeasonLengthTicks,
			ResetEveryTicks:                 spec.ResetEveryTicks,
			ResetNoticeTicks:                spec.ResetNoticeTicks,
			ObsRadius:                       tune.ObsRadius,
			Height:                          1,
			Seed:                            rtCfg.Seed + spec.SeedOffset,
			BoundaryR:                       spec.BoundaryR,
			SwitchCooldownTicks:             spec.SwitchCooldownTicks,
			AllowClaims:                     spec.AllowClaims,
			AllowMine:                       spec.AllowMine,
			AllowPlace:                      spec.AllowPlace,
			AllowLaws:                       spec.AllowLaws,
			AllowTrade:                      spec.AllowTrade,
			AllowBuild:                      spec.AllowBuild,
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
			logger.Fatalf("create world (%s): %v", spec.ID, err)
		}

		tickLog := persistlog.NewTickLogger(worldDir)
		auditLog := persistlog.NewAuditLogger(worldDir)
		defer tickLog.Close()
		defer auditLog.Close()
		w.SetTickLogger(multiTickLogger{a: tickLog, b: idx})
		w.SetAuditLogger(multiAuditLogger{a: auditLog, b: idx})

		snapCh := make(chan snapshot.SnapshotV1, 2)
		w.SetSnapshotSink(snapCh)
		go func(worldID, dir string, db *indexdb.SQLiteIndex) {
			for {
				select {
				case <-ctx.Done():
					return
				case snap := <-snapCh:
					path := filepath.Join(dir, "snapshots", fmt.Sprintf("%d.snap.zst", snap.Header.Tick))
					if err := snapshot.WriteSnapshot(path, snap); err != nil {
						logger.Printf("snapshot write (%s): %v", worldID, err)
						continue
					}
					if db != nil {
						db.RecordSnapshot(path, snap)
						db.RecordSnapshotState(snap)
						if season, archivedPath, ok, err := archive.ArchiveSeasonSnapshot(dir, path, snap); err != nil {
							logger.Printf("archive season snapshot (%s): %v", worldID, err)
						} else if ok {
							db.RecordSeason(season, snap.Header.Tick, archivedPath, snap.Seed)
						}
					} else {
						if _, _, _, err := archive.ArchiveSeasonSnapshot(dir, path, snap); err != nil {
							logger.Printf("archive season snapshot (%s): %v", worldID, err)
						}
					}
				}
			}
		}(spec.ID, worldDir, idx)

		go func(id string, ww *world.World) {
			if err := ww.Run(ctx); err != nil && err != context.Canceled {
				logger.Printf("world stopped (%s): %v", id, err)
			}
		}(spec.ID, w)

		runtimes[spec.ID] = &multiworld.Runtime{Spec: spec, World: w}
	}

	stateFile := filepath.Join(rtCfg.DataDir, "global", "state.json")
	mgr, err := multiworld.NewManager(cfg, runtimes, stateFile)
	if err != nil {
		logger.Fatalf("multiworld manager: %v", err)
	}
	defer mgr.Close()
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ctx2, cancel2 := context.WithTimeout(ctx, 2*time.Second)
				_ = mgr.RefreshOrgMeta(ctx2)
				cancel2()
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(200)
		_, _ = rw.Write([]byte("ok"))
	})
	mux.HandleFunc("/metrics", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "text/plain; version=0.0.4")
		for _, worldID := range mgr.WorldIDs() {
			rt := mgr.Runtime(worldID)
			if rt == nil || rt.World == nil {
				continue
			}
			m := rt.World.Metrics()
			fmt.Fprintf(rw, "voxelcraft_world_tick{world=%q} %d\n", worldID, rt.World.CurrentTick())
			fmt.Fprintf(rw, "voxelcraft_world_agents{world=%q} %d\n", worldID, m.Agents)
			fmt.Fprintf(rw, "voxelcraft_world_online_agents{world=%q} %d\n", worldID, m.Agents)
			fmt.Fprintf(rw, "voxelcraft_world_clients{world=%q} %d\n", worldID, m.Clients)
			fmt.Fprintf(rw, "voxelcraft_world_loaded_chunks{world=%q} %d\n", worldID, m.LoadedChunks)
			fmt.Fprintf(rw, "voxelcraft_world_reset_total{world=%q} %d\n", worldID, m.ResetTotal)
			resourceKeys := make([]string, 0, len(m.ResourceDensity))
			for resource := range m.ResourceDensity {
				resourceKeys = append(resourceKeys, resource)
			}
			sort.Strings(resourceKeys)
			for _, resource := range resourceKeys {
				fmt.Fprintf(rw, "voxelcraft_world_resource_density{world=%q,resource=%q} %.6f\n", worldID, resource, m.ResourceDensity[resource])
			}
		}
		for _, sm := range mgr.SwitchMetrics() {
			fmt.Fprintf(rw, "voxelcraft_world_switch_total{from=%q,to=%q,result=%q} %d\n", sm.From, sm.To, sm.Result, sm.Count)
		}
	})
	mux.HandleFunc("/admin/v1/worlds/state", func(rw http.ResponseWriter, r *http.Request) {
		if !isLoopbackRemote(r.RemoteAddr) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		out := map[string]any{}
		for _, worldID := range mgr.WorldIDs() {
			rt := mgr.Runtime(worldID)
			if rt == nil || rt.World == nil {
				continue
			}
			out[worldID] = map[string]any{
				"tick":    rt.World.CurrentTick(),
				"metrics": rt.World.Metrics(),
			}
		}
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(out)
	})
	mux.HandleFunc("/admin/v1/worlds/reset", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemote(r.RemoteAddr) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		worldID := r.URL.Query().Get("world")
		rt := mgr.Runtime(worldID)
		if rt == nil || rt.World == nil {
			http.Error(rw, "world not found", http.StatusNotFound)
			return
		}
		if !rt.Spec.AllowAdminReset {
			http.Error(rw, "reset forbidden for this world", http.StatusForbidden)
			return
		}
		// 1.0: trigger an immediate reset at tick boundary.
		ctx2, cancel2 := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel2()
		tick, err := rt.World.RequestReset(ctx2)
		rw.Header().Set("Content-Type", "application/json")
		if err != nil {
			rw.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(rw).Encode(map[string]any{"ok": false, "world": worldID, "tick": tick, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(rw).Encode(map[string]any{"ok": true, "world": worldID, "tick": tick, "note": "world reset completed"})
	})
	mux.HandleFunc("/admin/v1/worlds/", func(rw http.ResponseWriter, r *http.Request) {
		// Pattern: /admin/v1/worlds/{id}/reset
		if r.Method != http.MethodPost {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemote(r.RemoteAddr) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/admin/v1/worlds/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) != 2 || parts[1] != "reset" {
			http.NotFound(rw, r)
			return
		}
		worldID := parts[0]
		rt := mgr.Runtime(worldID)
		if rt == nil || rt.World == nil {
			http.Error(rw, "world not found", http.StatusNotFound)
			return
		}
		if !rt.Spec.AllowAdminReset {
			http.Error(rw, "reset forbidden for this world", http.StatusForbidden)
			return
		}
		ctx2, cancel2 := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel2()
		tick, err := rt.World.RequestReset(ctx2)
		rw.Header().Set("Content-Type", "application/json")
		if err != nil {
			rw.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(rw).Encode(map[string]any{"ok": false, "world": worldID, "tick": tick, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(rw).Encode(map[string]any{"ok": true, "world": worldID, "tick": tick, "note": "world reset completed"})
	})
	mux.HandleFunc("/admin/v1/agents/", func(rw http.ResponseWriter, r *http.Request) {
		// Pattern: /admin/v1/agents/{id}/move_world
		if r.Method != http.MethodPost {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemote(r.RemoteAddr) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/admin/v1/agents/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) != 2 || parts[1] != "move_world" {
			http.NotFound(rw, r)
			return
		}
		agentID := parts[0]
		targetWorld := strings.TrimSpace(r.URL.Query().Get("target_world"))
		if targetWorld == "" {
			http.Error(rw, "missing target_world", http.StatusBadRequest)
			return
		}
		ctx2, cancel2 := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel2()
		err := mgr.MoveAgentWorld(ctx2, agentID, targetWorld)
		rw.Header().Set("Content-Type", "application/json")
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(rw).Encode(map[string]any{"ok": false, "agent_id": agentID, "target_world": targetWorld, "error": err.Error()})
			return
		}
		_ = json.NewEncoder(rw).Encode(map[string]any{"ok": true, "agent_id": agentID, "target_world": targetWorld})
	})

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.HandleFunc("/v1/ws", ws.NewManagedServer(mgr, logger).Handler())

	srv := &http.Server{
		Addr:              rtCfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		_ = srv.Shutdown(ctx2)
	}()

	logger.Printf("multi-world mode listening on %s worlds=%v", rtCfg.Addr, mgr.WorldIDs())
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("ListenAndServe: %v", err)
	}
}
