package multiworld

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
	"voxelcraft.ai/internal/sim/world"
)

func newTestWorldForManager(t *testing.T, id string, seed int64) *world.World {
	t.Helper()
	root := findRepoRoot(t)
	cats, err := catalogs.Load(filepath.Join(root, "configs"))
	if err != nil {
		t.Fatalf("load catalogs: %v", err)
	}
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

func TestManagerSwitchWorld_DeniedWhenNotAtEntryPoint(t *testing.T) {
	wOver := newTestWorldForManager(t, "OVERWORLD", 10)
	wMine := newTestWorldForManager(t, "MINE_L1", 20)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = wOver.Run(ctx) }()
	go func() { _ = wMine.Run(ctx) }()

	cfg := Config{
		DefaultWorldID: "OVERWORLD",
		Worlds: []WorldSpec{
			{
				ID: "OVERWORLD", Type: "OVERWORLD", BoundaryR: 128, ResetEveryTicks: 12000, SwitchCooldownTicks: 1,
				EntryPointID: "over_far",
				EntryPoints: []EntryPointSpec{
					{ID: "over_far", X: 80, Z: 80, Radius: 2, Enabled: true},
				},
			},
			{
				ID: "MINE_L1", Type: "MINE_L1", BoundaryR: 128, ResetEveryTicks: 12000, SwitchCooldownTicks: 1,
				EntryPointID: "mine_gate",
				EntryPoints: []EntryPointSpec{
					{ID: "mine_gate", X: 0, Z: 0, Radius: 8, Enabled: true},
				},
			},
		},
		SwitchRoutes: []SwitchRouteSpec{
			{FromWorld: "OVERWORLD", ToWorld: "MINE_L1", FromEntryID: "over_far", ToEntryID: "mine_gate"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate cfg: %v", err)
	}
	mgr, err := NewManager(cfg, map[string]*Runtime{
		"OVERWORLD": {Spec: cfg.Worlds[0], World: wOver},
		"MINE_L1":   {Spec: cfg.Worlds[1], World: wMine},
	}, filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()
	out := make(chan []byte, 256)
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
			{ID: "I_SWITCH_DENY", Type: "SWITCH_WORLD", TargetWorldID: "MINE_L1"},
		},
	})
	if err != nil {
		t.Fatalf("route act: %v", err)
	}
	ev, _ := waitActionResult(t, out, "I_SWITCH_DENY", 3*time.Second)
	if ok, _ := ev["ok"].(bool); ok {
		t.Fatalf("expected denied switch, got %+v", ev)
	}
	if code, _ := ev["code"].(string); code != "E_WORLD_DENIED" {
		t.Fatalf("expected E_WORLD_DENIED, got %+v", ev)
	}
	denied := uint64(0)
	for _, sm := range mgr.SwitchMetrics() {
		if sm.From == "OVERWORLD" && sm.To == "MINE_L1" && sm.Result == "denied" {
			denied = sm.Count
			break
		}
	}
	if denied != 1 {
		t.Fatalf("expected denied switch metric=1, got %d", denied)
	}
}

func TestManagerSwitchWorld_SucceedsInsideEntryPoint(t *testing.T) {
	wOver := newTestWorldForManager(t, "OVERWORLD", 11)
	wMine := newTestWorldForManager(t, "MINE_L1", 21)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = wOver.Run(ctx) }()
	go func() { _ = wMine.Run(ctx) }()

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
			{
				ID: "MINE_L1", Type: "MINE_L1", BoundaryR: 128, ResetEveryTicks: 12000, SwitchCooldownTicks: 1,
				EntryPointID: "mine_gate",
				EntryPoints: []EntryPointSpec{
					{ID: "mine_gate", X: 0, Z: 0, Radius: 8, Enabled: true},
				},
			},
		},
		SwitchRoutes: []SwitchRouteSpec{
			{FromWorld: "OVERWORLD", ToWorld: "MINE_L1", FromEntryID: "over_spawn", ToEntryID: "mine_gate"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate cfg: %v", err)
	}
	mgr, err := NewManager(cfg, map[string]*Runtime{
		"OVERWORLD": {Spec: cfg.Worlds[0], World: wOver},
		"MINE_L1":   {Spec: cfg.Worlds[1], World: wMine},
	}, filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()
	out := make(chan []byte, 256)
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
			{ID: "I_SWITCH_OK", Type: "SWITCH_WORLD", TargetWorldID: "MINE_L1"},
		},
	})
	if err != nil {
		t.Fatalf("route act: %v", err)
	}
	ev, switchedObs := waitActionResult(t, out, "I_SWITCH_OK", 3*time.Second)
	if ok, _ := ev["ok"].(bool); !ok {
		t.Fatalf("expected successful switch, got %+v", ev)
	}
	if switchedObs.WorldID != "MINE_L1" {
		t.Fatalf("expected switched world obs, got %s", switchedObs.WorldID)
	}
	if got := ev["from_entry_id"]; got != "over_spawn" {
		t.Fatalf("expected from_entry_id=over_spawn, got %v", got)
	}
	if got := ev["to_entry_id"]; got != "mine_gate" {
		t.Fatalf("expected to_entry_id=mine_gate, got %v", got)
	}
}

func TestManagerSwitchWorld_SelectsMatchingEntryWhenEntryPointOmitted(t *testing.T) {
	wOver := newTestWorldForManager(t, "OVERWORLD", 12)
	wMine := newTestWorldForManager(t, "MINE_L1", 22)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = wOver.Run(ctx) }()
	go func() { _ = wMine.Run(ctx) }()

	cfg := Config{
		DefaultWorldID: "OVERWORLD",
		Worlds: []WorldSpec{
			{
				ID: "OVERWORLD", Type: "OVERWORLD", BoundaryR: 128, ResetEveryTicks: 12000, SwitchCooldownTicks: 1,
				EntryPointID: "b_near",
				EntryPoints: []EntryPointSpec{
					{ID: "a_far", X: 80, Z: 80, Radius: 3, Enabled: true},
					{ID: "b_near", X: 0, Z: 0, Radius: 16, Enabled: true},
				},
			},
			{
				ID: "MINE_L1", Type: "MINE_L1", BoundaryR: 128, ResetEveryTicks: 12000, SwitchCooldownTicks: 1,
				EntryPointID: "mine_gate",
				EntryPoints: []EntryPointSpec{
					{ID: "mine_gate", X: 0, Z: 0, Radius: 8, Enabled: true},
				},
			},
		},
		SwitchRoutes: []SwitchRouteSpec{
			{FromWorld: "OVERWORLD", ToWorld: "MINE_L1", FromEntryID: "a_far", ToEntryID: "mine_gate"},
			{FromWorld: "OVERWORLD", ToWorld: "MINE_L1", FromEntryID: "b_near", ToEntryID: "mine_gate"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate cfg: %v", err)
	}
	mgr, err := NewManager(cfg, map[string]*Runtime{
		"OVERWORLD": {Spec: cfg.Worlds[0], World: wOver},
		"MINE_L1":   {Spec: cfg.Worlds[1], World: wMine},
	}, filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()
	out := make(chan []byte, 256)
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
			{ID: "I_SWITCH_AUTO", Type: "SWITCH_WORLD", TargetWorldID: "MINE_L1"},
		},
	})
	if err != nil {
		t.Fatalf("route act: %v", err)
	}
	ev, switchedObs := waitActionResult(t, out, "I_SWITCH_AUTO", 3*time.Second)
	if ok, _ := ev["ok"].(bool); !ok {
		t.Fatalf("expected successful switch, got %+v", ev)
	}
	if switchedObs.WorldID != "MINE_L1" {
		t.Fatalf("expected switched world obs, got %s", switchedObs.WorldID)
	}
	if got := ev["from_entry_id"]; got != "b_near" {
		t.Fatalf("expected from_entry_id=b_near, got %v", got)
	}
}
