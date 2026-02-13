package main

import (
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

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/multiworld"
	"voxelcraft.ai/internal/sim/world"
)

func findRepoRootForServerTests(t *testing.T) string {
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
			t.Fatalf("could not locate go.mod from %s", dir)
		}
		dir = parent
	}
}

func newTestMultiWorldManagerForServer(t *testing.T) (*multiworld.Manager, func()) {
	t.Helper()
	root := findRepoRootForServerTests(t)
	cats, err := catalogs.Load(filepath.Join(root, "configs"))
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	newWorld := func(spec multiworld.WorldSpec, seed int64) *world.World {
		w, err := world.New(world.WorldConfig{
			ID:                  spec.ID,
			WorldType:           spec.Type,
			TickRateHz:          10,
			DayTicks:            6000,
			ObsRadius:           7,
			Height:              1,
			Seed:                seed,
			BoundaryR:           spec.BoundaryR,
			ResetEveryTicks:     spec.ResetEveryTicks,
			ResetNoticeTicks:    spec.ResetNoticeTicks,
			SwitchCooldownTicks: spec.SwitchCooldownTicks,
			AllowClaims:         spec.AllowClaims,
			AllowMine:           spec.AllowMine,
			AllowPlace:          spec.AllowPlace,
			AllowLaws:           spec.AllowLaws,
			AllowTrade:          spec.AllowTrade,
			AllowBuild:          spec.AllowBuild,
		}, cats)
		if err != nil {
			t.Fatalf("new world %s: %v", spec.ID, err)
		}
		return w
	}

	cfg := multiworld.Config{
		DefaultWorldID: "OVERWORLD",
		Worlds: []multiworld.WorldSpec{
			{
				ID: "OVERWORLD", Type: "OVERWORLD", BoundaryR: 128, ResetEveryTicks: 12000, ResetNoticeTicks: 300, SwitchCooldownTicks: 1,
				EntryPointID: "over_spawn", AllowAdminReset: false,
				EntryPoints: []multiworld.EntryPointSpec{
					{ID: "over_spawn", X: 0, Z: 0, Radius: 16, Enabled: true},
				},
				AllowClaims: true, AllowMine: true, AllowPlace: true, AllowLaws: true, AllowTrade: true, AllowBuild: true,
			},
			{
				ID: "MINE_L1", Type: "MINE_L1", BoundaryR: 128, ResetEveryTicks: 12000, ResetNoticeTicks: 300, SwitchCooldownTicks: 1,
				EntryPointID: "mine_gate", AllowAdminReset: true,
				EntryPoints: []multiworld.EntryPointSpec{
					{ID: "mine_gate", X: 0, Z: 0, Radius: 8, Enabled: true},
				},
				AllowClaims: false, AllowMine: true, AllowPlace: true, AllowLaws: false, AllowTrade: false, AllowBuild: false,
			},
		},
		SwitchRoutes: []multiworld.SwitchRouteSpec{
			{FromWorld: "OVERWORLD", ToWorld: "MINE_L1", FromEntryID: "over_spawn", ToEntryID: "mine_gate"},
			{FromWorld: "MINE_L1", ToWorld: "OVERWORLD", FromEntryID: "mine_gate", ToEntryID: "over_spawn"},
		},
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runtimes := map[string]*multiworld.Runtime{}
	for i, spec := range cfg.Worlds {
		w := newWorld(spec, int64(100+i))
		go func(ww *world.World) { _ = ww.Run(ctx) }(w)
		runtimes[spec.ID] = &multiworld.Runtime{Spec: spec, World: w}
	}

	mgr, err := multiworld.NewManager(cfg, runtimes, filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		cancel()
		t.Fatalf("new manager: %v", err)
	}
	stop := func() {
		mgr.Close()
		cancel()
	}
	return mgr, stop
}

func waitObsForServerTests(t *testing.T, out <-chan []byte, timeout time.Duration) protocol.ObsMsg {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting OBS")
		case b := <-out:
			var base protocol.BaseMessage
			if err := json.Unmarshal(b, &base); err != nil || base.Type != protocol.TypeObs {
				continue
			}
			var obs protocol.ObsMsg
			if err := json.Unmarshal(b, &obs); err != nil {
				continue
			}
			return obs
		}
	}
}

func waitActionResultForServerTests(t *testing.T, out <-chan []byte, ref string, timeout time.Duration) protocol.Event {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting ACTION_RESULT ref=%s", ref)
		case b := <-out:
			var base protocol.BaseMessage
			if err := json.Unmarshal(b, &base); err != nil || base.Type != protocol.TypeObs {
				continue
			}
			var obs protocol.ObsMsg
			if err := json.Unmarshal(b, &obs); err != nil {
				continue
			}
			for _, ev := range obs.Events {
				if typ, _ := ev["type"].(string); typ != "ACTION_RESULT" {
					continue
				}
				if gotRef, _ := ev["ref"].(string); gotRef == ref {
					return ev
				}
			}
		}
	}
}

func TestBuildMultiWorldMux_AdminResetAndLoopback(t *testing.T) {
	mgr, stop := newTestMultiWorldManagerForServer(t)
	defer stop()
	mux := buildMultiWorldMux(mgr, log.New(io.Discard, "", 0), nil, true, false)

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/worlds/state", nil)
	req.RemoteAddr = "8.8.8.8:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-loopback admin state, got %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/v1/worlds/OVERWORLD/reset", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for OVERWORLD reset, got %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/v1/worlds/MINE_L1/reset", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for MINE_L1 reset, got %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode reset response: %v", err)
	}
	if ok, _ := body["ok"].(bool); !ok {
		t.Fatalf("expected ok=true in reset response, got %+v", body)
	}
	if worldID, _ := body["world"].(string); worldID != "MINE_L1" {
		t.Fatalf("expected world=MINE_L1 in reset response, got %+v", body)
	}
}

func TestBuildMultiWorldMux_MetricsIncludesSwitchAndResourceDensity(t *testing.T) {
	mgr, stop := newTestMultiWorldManagerForServer(t)
	defer stop()
	mux := buildMultiWorldMux(mgr, log.New(io.Discard, "", 0), nil, true, false)

	out := make(chan []byte, 256)
	sess, _, err := mgr.Join("metrics_agent", true, out, "OVERWORLD")
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	obs := waitObsForServerTests(t, out, 3*time.Second)
	_, err = mgr.RouteAct(context.Background(), &sess, protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            obs.Tick,
		AgentID:         sess.AgentID,
		Instants: []protocol.InstantReq{
			{ID: "I_SWITCH_METRICS", Type: "SWITCH_WORLD", TargetWorldID: "MINE_L1"},
		},
	})
	if err != nil {
		t.Fatalf("switch route act: %v", err)
	}
	ev := waitActionResultForServerTests(t, out, "I_SWITCH_METRICS", 3*time.Second)
	if ok, _ := ev["ok"].(bool); !ok {
		t.Fatalf("switch failed unexpectedly: %+v", ev)
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `voxelcraft_world_switch_total{from="OVERWORLD",to="MINE_L1",result="ok"}`) {
		t.Fatalf("metrics missing switch_total line:\n%s", body)
	}
	if !strings.Contains(body, `voxelcraft_world_resource_density{world="OVERWORLD",resource="STONE"}`) {
		t.Fatalf("metrics missing resource density line:\n%s", body)
	}
}
