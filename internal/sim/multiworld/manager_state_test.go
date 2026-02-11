package multiworld

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"voxelcraft.ai/internal/sim/world"
)

func testManagerConfig() Config {
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
	cfg.Normalize()
	return cfg
}

func testRuntimes(t *testing.T) (map[string]*Runtime, func()) {
	t.Helper()
	wOver := newTestWorldForManager(t, "OVERWORLD", 31)
	wMine := newTestWorldForManager(t, "MINE_L1", 32)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = wOver.Run(ctx) }()
	go func() { _ = wMine.Run(ctx) }()
	return map[string]*Runtime{
		"OVERWORLD": {Spec: testManagerConfig().Worlds[0], World: wOver},
		"MINE_L1":   {Spec: testManagerConfig().Worlds[1], World: wMine},
	}, cancel
}

func TestManagerStateV2_PersistAndReload(t *testing.T) {
	runtimes, stop := testRuntimes(t)
	defer stop()

	cfg := testManagerConfig()
	statePath := filepath.Join(t.TempDir(), "state.json")
	mgr, err := NewManager(cfg, runtimes, statePath)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()

	mgr.updateResidency("A17", "MINE_L1", "resume_abc")
	mgr.recordSwitch("OVERWORLD", "MINE_L1", "ok")
	mgr.mergeOrgMetaFromTransfer(&world.OrgTransfer{
		OrgID:       "ORG000001",
		Kind:        world.OrgGuild,
		Name:        "Guild One",
		CreatedTick: 42,
		Members:     map[string]world.OrgRole{"A17": world.OrgLeader},
	})
	if err := mgr.FlushState(context.Background()); err != nil {
		t.Fatalf("flush state: %v", err)
	}

	reloaded, err := NewManager(cfg, runtimes, statePath)
	if err != nil {
		t.Fatalf("reload manager: %v", err)
	}
	defer reloaded.Close()

	if got := reloaded.AgentWorld("A17"); got != "MINE_L1" {
		t.Fatalf("agent residency not restored: got %q", got)
	}
	if got := reloaded.resumeToWorld["resume_abc"]; got != "MINE_L1" {
		t.Fatalf("resume residency not restored: got %q", got)
	}
	if got := reloaded.switchTotals[switchMetricKey{From: "OVERWORLD", To: "MINE_L1", Result: "ok"}]; got != 1 {
		t.Fatalf("switch metrics not restored: got %d", got)
	}
	meta, ok := reloaded.globalOrgMeta["ORG000001"]
	if !ok {
		t.Fatalf("org metadata not restored")
	}
	if meta.Name != "Guild One" || meta.Kind != world.OrgGuild {
		t.Fatalf("org metadata mismatch: %+v", meta)
	}
	if role := meta.Members["A17"]; role != world.OrgLeader {
		t.Fatalf("org membership mismatch: %q", role)
	}
}

func TestManagerState_CompatibleWithLegacyResidencyFile(t *testing.T) {
	runtimes, stop := testRuntimes(t)
	defer stop()
	cfg := testManagerConfig()
	base := t.TempDir()
	legacyPath := filepath.Join(base, "agent_residency.json")
	raw, _ := json.Marshal(persistedResidency{
		AgentToWorld:  map[string]string{"A9": "OVERWORLD"},
		ResumeToWorld: map[string]string{"resume_old": "OVERWORLD"},
	})
	if err := os.WriteFile(legacyPath, append(raw, '\n'), 0o644); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}

	mgr, err := NewManager(cfg, runtimes, filepath.Join(base, "state.json"))
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	defer mgr.Close()
	if got := mgr.AgentWorld("A9"); got != "OVERWORLD" {
		t.Fatalf("legacy agent residency not restored: %q", got)
	}
	if got := mgr.resumeToWorld["resume_old"]; got != "OVERWORLD" {
		t.Fatalf("legacy resume residency not restored: %q", got)
	}
}
