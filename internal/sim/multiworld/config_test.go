package multiworld

import "testing"

func TestLoad_WorldsYAML_AdminResetDefaults(t *testing.T) {
	cfg, err := Load("../../../configs/worlds.yaml")
	if err != nil {
		t.Fatalf("load worlds.yaml: %v", err)
	}
	specByID := map[string]WorldSpec{}
	for _, w := range cfg.Worlds {
		specByID[w.ID] = w
	}
	if specByID["OVERWORLD"].AllowAdminReset {
		t.Fatalf("OVERWORLD allow_admin_reset should default to false")
	}
	if !specByID["MINE_L1"].AllowAdminReset || !specByID["MINE_L2"].AllowAdminReset || !specByID["MINE_L3"].AllowAdminReset {
		t.Fatalf("mine worlds should default allow_admin_reset=true")
	}
	if specByID["CITY_HUB"].AllowAdminReset {
		t.Fatalf("CITY_HUB allow_admin_reset should default to false")
	}
	if len(cfg.SwitchRoutes) == 0 {
		t.Fatalf("expected switch_routes from config")
	}
}

func TestConfigNormalize_BackwardCompatibleRoutesAndEntryPoints(t *testing.T) {
	cfg := Config{
		DefaultWorldID: "A",
		Worlds: []WorldSpec{
			{ID: "A", Type: "A", BoundaryR: 64, ResetEveryTicks: 1000, SwitchCooldownTicks: 1},
			{ID: "B", Type: "B", BoundaryR: 64, ResetEveryTicks: 1000, SwitchCooldownTicks: 1},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate config: %v", err)
	}
	cfg.Normalize()
	if len(cfg.Worlds[0].EntryPoints) == 0 || len(cfg.Worlds[1].EntryPoints) == 0 {
		t.Fatalf("normalize should synthesize entry points")
	}
	if len(cfg.SwitchRoutes) != 2 {
		t.Fatalf("normalize should synthesize full-mesh routes: got %d want 2", len(cfg.SwitchRoutes))
	}
}
