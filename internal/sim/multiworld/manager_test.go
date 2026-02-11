package multiworld

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/world"
)

func waitObsMsg(t *testing.T, out <-chan []byte, timeout time.Duration) protocol.ObsMsg {
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

func waitActionResult(t *testing.T, out <-chan []byte, ref string, timeout time.Duration) (protocol.Event, protocol.ObsMsg) {
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
				typ, _ := ev["type"].(string)
				r, _ := ev["ref"].(string)
				if typ == "ACTION_RESULT" && r == ref {
					return ev, obs
				}
			}
		}
	}
}

func findRepoRoot(t *testing.T) string {
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
			t.Fatalf("could not find repo root from %s", dir)
		}
		dir = parent
	}
}

func TestManager_SwitchWorld(t *testing.T) {
	root := findRepoRoot(t)
	cats, err := catalogs.Load(filepath.Join(root, "configs"))
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}

	newWorld := func(id string, seed int64) *world.World {
		w, err := world.New(world.WorldConfig{
			ID:                  id,
			WorldType:           id,
			TickRateHz:          10,
			DayTicks:            6000,
			ObsRadius:           7,
			Height:              1,
			Seed:                seed,
			BoundaryR:           128,
			ResetEveryTicks:     12000,
			SwitchCooldownTicks: 1,
			AllowClaims:         true,
			AllowMine:           true,
			AllowPlace:          true,
			AllowLaws:           true,
			AllowTrade:          true,
			AllowBuild:          true,
		}, cats)
		if err != nil {
			t.Fatalf("new world %s: %v", id, err)
		}
		return w
	}

	wOver := newWorld("OVERWORLD", 11)
	wMine := newWorld("MINE_L1", 22)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = wOver.Run(ctx) }()
	go func() { _ = wMine.Run(ctx) }()

	cfg := Config{
		DefaultWorldID: "OVERWORLD",
		Worlds: []WorldSpec{
			{ID: "OVERWORLD", Type: "OVERWORLD", BoundaryR: 128, ResetEveryTicks: 12000, SwitchCooldownTicks: 1},
			{ID: "MINE_L1", Type: "MINE_L1", BoundaryR: 128, ResetEveryTicks: 12000, SwitchCooldownTicks: 1},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate cfg: %v", err)
	}
	mgr, err := NewManager(cfg, map[string]*Runtime{
		"OVERWORLD": {Spec: cfg.Worlds[0], World: wOver},
		"MINE_L1":   {Spec: cfg.Worlds[1], World: wMine},
	}, filepath.Join(t.TempDir(), "residency.json"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()

	out := make(chan []byte, 256)
	sess, joinResp, err := mgr.Join("agent", true, out, "OVERWORLD")
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if joinResp.Welcome.CurrentWorldID != "OVERWORLD" {
		t.Fatalf("welcome current_world_id=%q", joinResp.Welcome.CurrentWorldID)
	}

	obs1 := waitObsMsg(t, out, 3*time.Second)
	if obs1.WorldID != "OVERWORLD" {
		t.Fatalf("initial obs world_id=%q", obs1.WorldID)
	}

	_, err = mgr.RouteAct(context.Background(), &sess, protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            obs1.Tick,
		AgentID:         sess.AgentID,
		Instants: []protocol.InstantReq{
			{ID: "I_SWITCH", Type: "SWITCH_WORLD", TargetWorldID: "MINE_L1"},
		},
	})
	if err != nil {
		t.Fatalf("switch act: %v", err)
	}
	if sess.CurrentWorld != "MINE_L1" {
		t.Fatalf("session current_world=%q", sess.CurrentWorld)
	}

	found := false
	deadline := time.After(3 * time.Second)
	for !found {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting switched obs")
		case b := <-out:
			var base protocol.BaseMessage
			if err := json.Unmarshal(b, &base); err != nil || base.Type != protocol.TypeObs {
				continue
			}
			var obs protocol.ObsMsg
			if err := json.Unmarshal(b, &obs); err != nil {
				continue
			}
			if obs.WorldID == "MINE_L1" {
				found = true
			}
		}
	}

	metrics := mgr.SwitchMetrics()
	okCount := uint64(0)
	for _, m := range metrics {
		if m.From == "OVERWORLD" && m.To == "MINE_L1" && m.Result == "ok" {
			okCount = m.Count
			break
		}
	}
	if okCount != 1 {
		t.Fatalf("switch metrics missing ok counter, got=%d metrics=%+v", okCount, metrics)
	}
}

func TestManager_SwitchWorld_MetricsOnFailure(t *testing.T) {
	root := findRepoRoot(t)
	cats, err := catalogs.Load(filepath.Join(root, "configs"))
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	w, err := world.New(world.WorldConfig{
		ID:                  "OVERWORLD",
		WorldType:           "OVERWORLD",
		TickRateHz:          10,
		DayTicks:            6000,
		ObsRadius:           7,
		Height:              1,
		Seed:                11,
		BoundaryR:           128,
		ResetEveryTicks:     12000,
		SwitchCooldownTicks: 1,
		AllowClaims:         true,
		AllowMine:           true,
		AllowPlace:          true,
		AllowLaws:           true,
		AllowTrade:          true,
		AllowBuild:          true,
	}, cats)
	if err != nil {
		t.Fatalf("new world: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = w.Run(ctx) }()

	cfg := Config{
		DefaultWorldID: "OVERWORLD",
		Worlds: []WorldSpec{
			{ID: "OVERWORLD", Type: "OVERWORLD", BoundaryR: 128, ResetEveryTicks: 12000, SwitchCooldownTicks: 1},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate cfg: %v", err)
	}
	mgr, err := NewManager(cfg, map[string]*Runtime{
		"OVERWORLD": {Spec: cfg.Worlds[0], World: w},
	}, filepath.Join(t.TempDir(), "residency.json"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()
	out := make(chan []byte, 128)
	sess, _, err := mgr.Join("agent", true, out, "OVERWORLD")
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	obs := waitObsMsg(t, out, 3*time.Second)
	_, err = mgr.RouteAct(context.Background(), &sess, protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            obs.Tick,
		AgentID:         sess.AgentID,
		Instants: []protocol.InstantReq{
			{ID: "I_SWITCH_BAD", Type: "SWITCH_WORLD", TargetWorldID: "MINE_L9"},
		},
	})
	if err != nil {
		t.Fatalf("route act unexpected error: %v", err)
	}
	ev, _ := waitActionResult(t, out, "I_SWITCH_BAD", 3*time.Second)
	if ok, _ := ev["ok"].(bool); ok {
		t.Fatalf("expected switch failure, got %+v", ev)
	}
	metrics := mgr.SwitchMetrics()
	failCount := uint64(0)
	for _, m := range metrics {
		if m.From == "OVERWORLD" && m.To == "MINE_L9" && m.Result == "world_not_found" {
			failCount = m.Count
			break
		}
	}
	if failCount != 1 {
		t.Fatalf("switch failure metric mismatch: got=%d metrics=%+v", failCount, metrics)
	}
}

func TestManager_SwitchWorld_OrgMembershipAndTreasuryIsolation(t *testing.T) {
	root := findRepoRoot(t)
	cats, err := catalogs.Load(filepath.Join(root, "configs"))
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
	newWorld := func(id string, seed int64) *world.World {
		w, err := world.New(world.WorldConfig{
			ID:                  id,
			WorldType:           id,
			TickRateHz:          10,
			DayTicks:            6000,
			ObsRadius:           7,
			Height:              1,
			Seed:                seed,
			BoundaryR:           128,
			ResetEveryTicks:     12000,
			SwitchCooldownTicks: 1,
			AllowClaims:         true,
			AllowMine:           true,
			AllowPlace:          true,
			AllowLaws:           true,
			AllowTrade:          true,
			AllowBuild:          true,
		}, cats)
		if err != nil {
			t.Fatalf("new world %s: %v", id, err)
		}
		return w
	}
	wOver := newWorld("OVERWORLD", 101)
	wMine := newWorld("MINE_L1", 202)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = wOver.Run(ctx) }()
	go func() { _ = wMine.Run(ctx) }()

	cfg := Config{
		DefaultWorldID: "OVERWORLD",
		Worlds: []WorldSpec{
			{ID: "OVERWORLD", Type: "OVERWORLD", BoundaryR: 128, ResetEveryTicks: 12000, SwitchCooldownTicks: 1},
			{ID: "MINE_L1", Type: "MINE_L1", BoundaryR: 128, ResetEveryTicks: 12000, SwitchCooldownTicks: 1},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate cfg: %v", err)
	}
	mgr, err := NewManager(cfg, map[string]*Runtime{
		"OVERWORLD": {Spec: cfg.Worlds[0], World: wOver},
		"MINE_L1":   {Spec: cfg.Worlds[1], World: wMine},
	}, filepath.Join(t.TempDir(), "residency.json"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()

	out := make(chan []byte, 512)
	sess, _, err := mgr.Join("org-agent", true, out, "OVERWORLD")
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	obs := waitObsMsg(t, out, 3*time.Second)

	// Create org in OVERWORLD.
	_, err = mgr.RouteAct(context.Background(), &sess, protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            obs.Tick,
		AgentID:         sess.AgentID,
		Instants: []protocol.InstantReq{
			{ID: "I_CREATE", Type: "CREATE_ORG", OrgKind: "GUILD", OrgName: "CrossWorldGuild"},
		},
	})
	if err != nil {
		t.Fatalf("create org act: %v", err)
	}
	evCreate, obs := waitActionResult(t, out, "I_CREATE", 3*time.Second)
	if ok, _ := evCreate["ok"].(bool); !ok {
		t.Fatalf("create org failed: %+v", evCreate)
	}
	orgID, _ := evCreate["org_id"].(string)
	if orgID == "" {
		t.Fatalf("missing org_id in create result: %+v", evCreate)
	}

	// Seed OVERWORLD treasury with 2 PLANK.
	_, err = mgr.RouteAct(context.Background(), &sess, protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            obs.Tick,
		AgentID:         sess.AgentID,
		Instants: []protocol.InstantReq{
			{ID: "I_DEPOSIT_OVER", Type: "ORG_DEPOSIT", OrgID: orgID, ItemID: "PLANK", Count: 2},
		},
	})
	if err != nil {
		t.Fatalf("deposit overworld act: %v", err)
	}
	evDepOver, obs := waitActionResult(t, out, "I_DEPOSIT_OVER", 3*time.Second)
	if ok, _ := evDepOver["ok"].(bool); !ok {
		t.Fatalf("overworld deposit failed: %+v", evDepOver)
	}

	// Switch to mine world.
	_, err = mgr.RouteAct(context.Background(), &sess, protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            obs.Tick,
		AgentID:         sess.AgentID,
		Instants: []protocol.InstantReq{
			{ID: "I_SWITCH", Type: "SWITCH_WORLD", TargetWorldID: "MINE_L1"},
		},
	})
	if err != nil {
		t.Fatalf("switch act: %v", err)
	}
	evSwitch, obs := waitActionResult(t, out, "I_SWITCH", 3*time.Second)
	if ok, _ := evSwitch["ok"].(bool); !ok {
		t.Fatalf("switch failed: %+v", evSwitch)
	}
	if obs.WorldID != "MINE_L1" || sess.CurrentWorld != "MINE_L1" {
		t.Fatalf("expected switched to MINE_L1, obs=%s sess=%s", obs.WorldID, sess.CurrentWorld)
	}

	// Deposit in MINE_L1 should work (org metadata/membership carried over).
	_, err = mgr.RouteAct(context.Background(), &sess, protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            obs.Tick,
		AgentID:         sess.AgentID,
		Instants: []protocol.InstantReq{
			{ID: "I_DEPOSIT_MINE", Type: "ORG_DEPOSIT", OrgID: orgID, ItemID: "PLANK", Count: 1},
		},
	})
	if err != nil {
		t.Fatalf("deposit mine act: %v", err)
	}
	evDepMine, obs := waitActionResult(t, out, "I_DEPOSIT_MINE", 3*time.Second)
	if ok, _ := evDepMine["ok"].(bool); !ok {
		t.Fatalf("mine deposit failed: %+v", evDepMine)
	}

	// Treasury is world-isolated: MINE_L1 has 1, so withdrawing 2 must fail.
	_, err = mgr.RouteAct(context.Background(), &sess, protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            obs.Tick,
		AgentID:         sess.AgentID,
		Instants: []protocol.InstantReq{
			{ID: "I_WITHDRAW_2", Type: "ORG_WITHDRAW", OrgID: orgID, ItemID: "PLANK", Count: 2},
		},
	})
	if err != nil {
		t.Fatalf("withdraw 2 act: %v", err)
	}
	evWd2, obs := waitActionResult(t, out, "I_WITHDRAW_2", 3*time.Second)
	if ok, _ := evWd2["ok"].(bool); ok {
		t.Fatalf("withdraw 2 unexpectedly succeeded: %+v", evWd2)
	}
	if code, _ := evWd2["code"].(string); code != "E_NO_RESOURCE" {
		t.Fatalf("withdraw 2 expected E_NO_RESOURCE, got %+v", evWd2)
	}

	// Withdrawing 1 should succeed.
	_, err = mgr.RouteAct(context.Background(), &sess, protocol.ActMsg{
		Type:            protocol.TypeAct,
		ProtocolVersion: protocol.Version,
		Tick:            obs.Tick,
		AgentID:         sess.AgentID,
		Instants: []protocol.InstantReq{
			{ID: "I_WITHDRAW_1", Type: "ORG_WITHDRAW", OrgID: orgID, ItemID: "PLANK", Count: 1},
		},
	})
	if err != nil {
		t.Fatalf("withdraw 1 act: %v", err)
	}
	evWd1, _ := waitActionResult(t, out, "I_WITHDRAW_1", 3*time.Second)
	if ok, _ := evWd1["ok"].(bool); !ok {
		t.Fatalf("withdraw 1 failed: %+v", evWd1)
	}
}
