package multiworld

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"voxelcraft.ai/internal/protocol"
)

func countAgentResidency(t *testing.T, runtimes map[string]*Runtime, agentID string) int {
	t.Helper()
	n := 0
	for _, rt := range runtimes {
		if rt == nil || rt.World == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		_, err := rt.World.RequestAgentPos(ctx, agentID)
		cancel()
		if err == nil {
			n++
		}
	}
	return n
}

func TestManagerSwitchWorld_ConcurrentRequestsKeepSingleResidency(t *testing.T) {
	wOver := newTestWorldForManager(t, "OVERWORLD", 61)
	wMine := newTestWorldForManager(t, "MINE_L1", 62)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = wOver.Run(ctx) }()
	go func() { _ = wMine.Run(ctx) }()

	cfg := testManagerConfig()
	runtimes := map[string]*Runtime{
		"OVERWORLD": {Spec: cfg.Worlds[0], World: wOver},
		"MINE_L1":   {Spec: cfg.Worlds[1], World: wMine},
	}
	mgr, err := NewManager(cfg, runtimes, filepathJoinTemp(t, "state.json"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()

	out := make(chan []byte, 256)
	sess, _, err := mgr.Join("concurrent_switch", true, out, "OVERWORLD")
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	obs := waitObsMsg(t, out, 3*time.Second)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		s := sess
		_, _ = mgr.RouteAct(context.Background(), &s, protocol.ActMsg{
			Type:            protocol.TypeAct,
			ProtocolVersion: protocol.Version,
			Tick:            obs.Tick,
			AgentID:         sess.AgentID,
			Instants: []protocol.InstantReq{
				{ID: "I_SWITCH_A", Type: "SWITCH_WORLD", TargetWorldID: "MINE_L1"},
			},
		})
	}()
	go func() {
		defer wg.Done()
		s := sess
		_, _ = mgr.RouteAct(context.Background(), &s, protocol.ActMsg{
			Type:            protocol.TypeAct,
			ProtocolVersion: protocol.Version,
			Tick:            obs.Tick,
			AgentID:         sess.AgentID,
			Instants: []protocol.InstantReq{
				{ID: "I_SWITCH_B", Type: "SWITCH_WORLD", TargetWorldID: "MINE_L1"},
			},
		})
	}()
	wg.Wait()

	worldID := mgr.AgentWorld(sess.AgentID)
	if worldID != "MINE_L1" {
		t.Fatalf("agent residency drifted, want MINE_L1 got %q", worldID)
	}
	if got := countAgentResidency(t, runtimes, sess.AgentID); got != 1 {
		t.Fatalf("agent should exist in exactly one world runtime, got=%d", got)
	}
}

func TestManagerRouteAct_ExpectedWorldMismatchKeepsResidency(t *testing.T) {
	wOver := newTestWorldForManager(t, "OVERWORLD", 71)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = wOver.Run(ctx) }()

	cfg := Config{
		DefaultWorldID: "OVERWORLD",
		Worlds: []WorldSpec{
			{
				ID: "OVERWORLD", Type: "OVERWORLD", BoundaryR: 128, ResetEveryTicks: 12000, SwitchCooldownTicks: 1,
				EntryPointID: "over_spawn",
				EntryPoints: []EntryPointSpec{
					{ID: "over_spawn", X: 0, Z: 0, Radius: 16, Enabled: true},
				},
			},
		},
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate cfg: %v", err)
	}
	runtimes := map[string]*Runtime{
		"OVERWORLD": {Spec: cfg.Worlds[0], World: wOver},
	}
	mgr, err := NewManager(cfg, runtimes, filepathJoinTemp(t, "state.json"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()

	out := make(chan []byte, 256)
	sess, _, err := mgr.Join("expected_world_guard", true, out, "OVERWORLD")
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	obs := waitObsMsg(t, out, 3*time.Second)

	cur, err := mgr.RouteAct(context.Background(), &sess, protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            obs.Tick,
		AgentID:         sess.AgentID,
		ExpectedWorldID: "MINE_L1",
	})
	if err != nil {
		t.Fatalf("route act mismatch guard: %v", err)
	}
	if cur != "OVERWORLD" {
		t.Fatalf("unexpected current world return: %q", cur)
	}

	ev, _ := waitActionResult(t, out, "ACT", 3*time.Second)
	if ok, _ := ev["ok"].(bool); ok {
		t.Fatalf("expected ACT guard failure event, got %+v", ev)
	}
	if code, _ := ev["code"].(string); code != "E_WORLD_BUSY" {
		t.Fatalf("expected E_WORLD_BUSY, got %+v", ev)
	}
	if sess.CurrentWorld != "OVERWORLD" {
		t.Fatalf("session world drifted: %q", sess.CurrentWorld)
	}
	if got := mgr.AgentWorld(sess.AgentID); got != "OVERWORLD" {
		t.Fatalf("manager residency drifted: %q", got)
	}
}

func TestManagerSwitchWorld_ConcurrentBurstNoResidencyLeak(t *testing.T) {
	wOver := newTestWorldForManager(t, "OVERWORLD", 81)
	wMine := newTestWorldForManager(t, "MINE_L1", 82)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = wOver.Run(ctx) }()
	go func() { _ = wMine.Run(ctx) }()

	cfg := Config{
		DefaultWorldID: "OVERWORLD",
		Worlds: []WorldSpec{
			{
				ID: "OVERWORLD", Type: "OVERWORLD", BoundaryR: 128, ResetEveryTicks: 12000, SwitchCooldownTicks: 0,
				EntryPointID: "over_spawn",
				EntryPoints: []EntryPointSpec{
					{ID: "over_spawn", X: 0, Z: 0, Radius: 16, Enabled: true},
				},
			},
			{
				ID: "MINE_L1", Type: "MINE_L1", BoundaryR: 128, ResetEveryTicks: 12000, SwitchCooldownTicks: 0,
				EntryPointID: "mine_gate",
				EntryPoints: []EntryPointSpec{
					{ID: "mine_gate", X: 0, Z: 0, Radius: 16, Enabled: true},
				},
			},
		},
		SwitchRoutes: []SwitchRouteSpec{
			{FromWorld: "OVERWORLD", ToWorld: "MINE_L1", FromEntryID: "over_spawn", ToEntryID: "mine_gate"},
			{FromWorld: "MINE_L1", ToWorld: "OVERWORLD", FromEntryID: "mine_gate", ToEntryID: "over_spawn"},
		},
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate cfg: %v", err)
	}
	runtimes := map[string]*Runtime{
		"OVERWORLD": {Spec: cfg.Worlds[0], World: wOver},
		"MINE_L1":   {Spec: cfg.Worlds[1], World: wMine},
	}
	mgr, err := NewManager(cfg, runtimes, filepathJoinTemp(t, "state.json"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()

	out := make(chan []byte, 512)
	sess, _, err := mgr.Join("burst_switch", true, out, "OVERWORLD")
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	_ = waitObsMsg(t, out, 3*time.Second)

	for round := 0; round < 8; round++ {
		current := mgr.AgentWorld(sess.AgentID)
		target := "MINE_L1"
		if current == "MINE_L1" {
			target = "OVERWORLD"
		}
		sess.CurrentWorld = current

		var wg sync.WaitGroup
		errCh := make(chan error, 4)
		for i := 0; i < 4; i++ {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				local := sess
				_, err := mgr.RouteAct(context.Background(), &local, protocol.ActMsg{
					Type:            protocol.TypeAct,
					ProtocolVersion: protocol.Version,
					Tick:            runtimes[current].World.CurrentTick(),
					AgentID:         sess.AgentID,
					ExpectedWorldID: current,
					Instants: []protocol.InstantReq{
						{
							ID:            fmt.Sprintf("I_BURST_%d_%d", round, i),
							Type:          "SWITCH_WORLD",
							TargetWorldID: target,
						},
					},
				})
				if err != nil {
					errCh <- err
				}
			}()
		}
		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				t.Fatalf("route act failed in round=%d: %v", round, err)
			}
		}

		if got := mgr.AgentWorld(sess.AgentID); got != target {
			t.Fatalf("round=%d expected world=%s got=%s", round, target, got)
		}
		if got := countAgentResidency(t, runtimes, sess.AgentID); got != 1 {
			t.Fatalf("round=%d residency leak: got=%d", round, got)
		}
	}
}

func filepathJoinTemp(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}
