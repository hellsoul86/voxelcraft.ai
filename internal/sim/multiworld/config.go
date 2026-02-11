package multiworld

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"voxelcraft.ai/internal/protocol"
)

type Config struct {
	DefaultWorldID string            `yaml:"default_world_id"`
	Worlds         []WorldSpec       `yaml:"worlds"`
	SwitchRoutes   []SwitchRouteSpec `yaml:"switch_routes,omitempty"`
}

type WorldSpec struct {
	ID                  string           `yaml:"id"`
	Type                string           `yaml:"type"`
	SeedOffset          int64            `yaml:"seed_offset"`
	BoundaryR           int              `yaml:"boundary_r"`
	ResetEveryTicks     int              `yaml:"reset_every_ticks"`
	ResetNoticeTicks    int              `yaml:"reset_notice_ticks"`
	SwitchCooldownTicks int              `yaml:"switch_cooldown_ticks"`
	EntryPointID        string           `yaml:"entry_point_id"`
	RequiresPermit      bool             `yaml:"requires_permit"`
	AllowAdminReset     bool             `yaml:"allow_admin_reset"`
	EntryPoints         []EntryPointSpec `yaml:"entry_points,omitempty"`

	AllowClaims bool `yaml:"allow_claims"`
	AllowMine   bool `yaml:"allow_mine"`
	AllowPlace  bool `yaml:"allow_place"`
	AllowLaws   bool `yaml:"allow_laws"`
	AllowTrade  bool `yaml:"allow_trade"`
	AllowBuild  bool `yaml:"allow_build"`
}

type EntryPointSpec struct {
	ID      string `yaml:"id"`
	X       int    `yaml:"x"`
	Z       int    `yaml:"z"`
	Radius  int    `yaml:"radius"`
	Enabled bool   `yaml:"enabled"`
}

type SwitchRouteSpec struct {
	FromWorld      string `yaml:"from_world"`
	ToWorld        string `yaml:"to_world"`
	FromEntryID    string `yaml:"from_entry_id"`
	ToEntryID      string `yaml:"to_entry_id"`
	RequiresPermit bool   `yaml:"requires_permit"`
}

func Load(path string) (Config, error) {
	cfg := defaults()
	if strings.TrimSpace(path) == "" {
		cfg.Normalize()
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("worlds.yaml: %w", err)
	}
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("worlds.yaml: %w", err)
	}
	return cfg, nil
}

func defaults() Config {
	return Config{
		DefaultWorldID: "OVERWORLD",
		Worlds: []WorldSpec{
			{
				ID:                  "OVERWORLD",
				Type:                "OVERWORLD",
				BoundaryR:           4000,
				ResetEveryTicks:     42000,
				ResetNoticeTicks:    0,
				SwitchCooldownTicks: 150,
				EntryPointID:        "overworld_spawn",
				AllowAdminReset:     false,
				EntryPoints: []EntryPointSpec{
					{ID: "overworld_spawn", X: 0, Z: 0, Radius: 8, Enabled: true},
				},
				AllowClaims: true,
				AllowMine:   true,
				AllowPlace:  true,
				AllowLaws:   true,
				AllowTrade:  true,
				AllowBuild:  true,
			},
			{
				ID:                  "MINE_L1",
				Type:                "MINE_L1",
				BoundaryR:           1200,
				ResetEveryTicks:     3000,
				ResetNoticeTicks:    300,
				SwitchCooldownTicks: 150,
				EntryPointID:        "mine_l1_gate",
				AllowAdminReset:     true,
				EntryPoints: []EntryPointSpec{
					{ID: "mine_l1_gate", X: 0, Z: 0, Radius: 8, Enabled: true},
				},
				AllowClaims: false,
				AllowMine:   true,
				AllowPlace:  true,
				AllowLaws:   false,
				AllowTrade:  false,
				AllowBuild:  false,
			},
			{
				ID:                  "MINE_L2",
				Type:                "MINE_L2",
				BoundaryR:           1000,
				ResetEveryTicks:     6000,
				ResetNoticeTicks:    300,
				SwitchCooldownTicks: 150,
				EntryPointID:        "mine_l2_gate",
				AllowAdminReset:     true,
				EntryPoints: []EntryPointSpec{
					{ID: "mine_l2_gate", X: 0, Z: 0, Radius: 8, Enabled: true},
				},
				AllowClaims: false,
				AllowMine:   true,
				AllowPlace:  true,
				AllowLaws:   false,
				AllowTrade:  false,
				AllowBuild:  false,
			},
			{
				ID:                  "MINE_L3",
				Type:                "MINE_L3",
				BoundaryR:           800,
				ResetEveryTicks:     12000,
				ResetNoticeTicks:    300,
				SwitchCooldownTicks: 150,
				EntryPointID:        "mine_l3_gate",
				AllowAdminReset:     true,
				EntryPoints: []EntryPointSpec{
					{ID: "mine_l3_gate", X: 0, Z: 0, Radius: 8, Enabled: true},
				},
				AllowClaims: false,
				AllowMine:   true,
				AllowPlace:  true,
				AllowLaws:   false,
				AllowTrade:  false,
				AllowBuild:  false,
			},
			{
				ID:                  "CITY_HUB",
				Type:                "CITY_HUB",
				BoundaryR:           600,
				ResetEveryTicks:     42000,
				ResetNoticeTicks:    0,
				SwitchCooldownTicks: 150,
				EntryPointID:        "city_gate",
				AllowAdminReset:     false,
				EntryPoints: []EntryPointSpec{
					{ID: "city_gate", X: 0, Z: 0, Radius: 8, Enabled: true},
				},
				AllowClaims: true,
				AllowMine:   false,
				AllowPlace:  true,
				AllowLaws:   true,
				AllowTrade:  true,
				AllowBuild:  true,
			},
		},
		SwitchRoutes: []SwitchRouteSpec{
			{FromWorld: "OVERWORLD", ToWorld: "MINE_L1", FromEntryID: "overworld_spawn", ToEntryID: "mine_l1_gate"},
			{FromWorld: "MINE_L1", ToWorld: "OVERWORLD", FromEntryID: "mine_l1_gate", ToEntryID: "overworld_spawn"},
			{FromWorld: "MINE_L1", ToWorld: "MINE_L2", FromEntryID: "mine_l1_gate", ToEntryID: "mine_l2_gate"},
			{FromWorld: "MINE_L2", ToWorld: "MINE_L1", FromEntryID: "mine_l2_gate", ToEntryID: "mine_l1_gate"},
			{FromWorld: "MINE_L2", ToWorld: "MINE_L3", FromEntryID: "mine_l2_gate", ToEntryID: "mine_l3_gate"},
			{FromWorld: "MINE_L3", ToWorld: "MINE_L2", FromEntryID: "mine_l3_gate", ToEntryID: "mine_l2_gate"},
			{FromWorld: "OVERWORLD", ToWorld: "CITY_HUB", FromEntryID: "overworld_spawn", ToEntryID: "city_gate"},
			{FromWorld: "CITY_HUB", ToWorld: "OVERWORLD", FromEntryID: "city_gate", ToEntryID: "overworld_spawn"},
		},
	}
}

func (c *Config) Normalize() {
	if c == nil {
		return
	}
	for i := range c.Worlds {
		if len(c.Worlds[i].EntryPoints) == 0 {
			id := strings.TrimSpace(c.Worlds[i].EntryPointID)
			if id == "" {
				id = strings.ToLower(c.Worlds[i].ID) + "_entry"
			}
			c.Worlds[i].EntryPoints = []EntryPointSpec{
				{ID: id, X: 0, Z: 0, Radius: 8, Enabled: true},
			}
		}
		for j := range c.Worlds[i].EntryPoints {
			if c.Worlds[i].EntryPoints[j].Radius <= 0 {
				c.Worlds[i].EntryPoints[j].Radius = 1
			}
			// default: enabled
			if !c.Worlds[i].EntryPoints[j].Enabled && len(c.Worlds[i].EntryPoints) == 1 {
				c.Worlds[i].EntryPoints[j].Enabled = true
			}
		}
		if strings.TrimSpace(c.Worlds[i].EntryPointID) == "" {
			c.Worlds[i].EntryPointID = c.Worlds[i].EntryPoints[0].ID
		}
	}
	if len(c.SwitchRoutes) == 0 {
		// Backward-compatible default: allow switching between any two worlds via their primary entry points.
		for i := range c.Worlds {
			for j := range c.Worlds {
				if i == j {
					continue
				}
				c.SwitchRoutes = append(c.SwitchRoutes, SwitchRouteSpec{
					FromWorld:   c.Worlds[i].ID,
					ToWorld:     c.Worlds[j].ID,
					FromEntryID: c.Worlds[i].EntryPointID,
					ToEntryID:   c.Worlds[j].EntryPointID,
				})
			}
		}
	}
}

func (c Config) Validate() error {
	c.Normalize()
	if len(c.Worlds) == 0 {
		return fmt.Errorf("worlds must not be empty")
	}
	seen := map[string]bool{}
	entryByWorld := map[string]map[string]bool{}
	for _, w := range c.Worlds {
		if strings.TrimSpace(w.ID) == "" {
			return fmt.Errorf("world id must not be empty")
		}
		if seen[w.ID] {
			return fmt.Errorf("duplicate world id: %s", w.ID)
		}
		seen[w.ID] = true
		if w.BoundaryR <= 0 {
			return fmt.Errorf("world %s boundary_r must be > 0", w.ID)
		}
		if w.ResetEveryTicks <= 0 {
			return fmt.Errorf("world %s reset_every_ticks must be > 0", w.ID)
		}
		if w.SwitchCooldownTicks < 0 {
			return fmt.Errorf("world %s switch_cooldown_ticks must be >= 0", w.ID)
		}
		if w.ResetNoticeTicks < 0 || w.ResetNoticeTicks >= w.ResetEveryTicks {
			return fmt.Errorf("world %s reset_notice_ticks must be in [0, reset_every_ticks)", w.ID)
		}
		if len(w.EntryPoints) == 0 {
			return fmt.Errorf("world %s must define at least one entry point", w.ID)
		}
		ids := map[string]bool{}
		for _, ep := range w.EntryPoints {
			epID := strings.TrimSpace(ep.ID)
			if epID == "" {
				return fmt.Errorf("world %s has empty entry point id", w.ID)
			}
			if ids[epID] {
				return fmt.Errorf("world %s duplicate entry point id: %s", w.ID, epID)
			}
			ids[epID] = true
			if ep.Radius <= 0 {
				return fmt.Errorf("world %s entry point %s radius must be > 0", w.ID, epID)
			}
		}
		entryByWorld[w.ID] = ids
		if strings.TrimSpace(w.EntryPointID) == "" {
			return fmt.Errorf("world %s entry_point_id must not be empty", w.ID)
		}
		if !ids[w.EntryPointID] {
			return fmt.Errorf("world %s entry_point_id %q not found in entry_points", w.ID, w.EntryPointID)
		}
	}
	if c.DefaultWorldID == "" {
		return fmt.Errorf("default_world_id must not be empty")
	}
	if !seen[c.DefaultWorldID] {
		return fmt.Errorf("default_world_id %q not found in worlds", c.DefaultWorldID)
	}
	for i, r := range c.SwitchRoutes {
		if strings.TrimSpace(r.FromWorld) == "" || strings.TrimSpace(r.ToWorld) == "" {
			return fmt.Errorf("switch_routes[%d] missing from_world/to_world", i)
		}
		if !seen[r.FromWorld] {
			return fmt.Errorf("switch_routes[%d] from_world %q not found", i, r.FromWorld)
		}
		if !seen[r.ToWorld] {
			return fmt.Errorf("switch_routes[%d] to_world %q not found", i, r.ToWorld)
		}
		if strings.TrimSpace(r.FromEntryID) == "" || strings.TrimSpace(r.ToEntryID) == "" {
			return fmt.Errorf("switch_routes[%d] missing from_entry_id/to_entry_id", i)
		}
		if !entryByWorld[r.FromWorld][r.FromEntryID] {
			return fmt.Errorf("switch_routes[%d] from_entry_id %q not found in %s", i, r.FromEntryID, r.FromWorld)
		}
		if !entryByWorld[r.ToWorld][r.ToEntryID] {
			return fmt.Errorf("switch_routes[%d] to_entry_id %q not found in %s", i, r.ToEntryID, r.ToWorld)
		}
	}
	return nil
}

func (c Config) Manifest() []protocol.WorldRef {
	out := make([]protocol.WorldRef, 0, len(c.Worlds))
	for _, w := range c.Worlds {
		out = append(out, protocol.WorldRef{
			WorldID:          w.ID,
			WorldType:        w.Type,
			EntryPointID:     w.EntryPointID,
			RequiresPermit:   w.RequiresPermit,
			SwitchCooldown:   w.SwitchCooldownTicks,
			ResetEveryTicks:  w.ResetEveryTicks,
			ResetNoticeTicks: w.ResetNoticeTicks,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].WorldID < out[j].WorldID })
	return out
}

func (c Config) WorldSpecByID(id string) (WorldSpec, bool) {
	for _, w := range c.Worlds {
		if w.ID == id {
			return w, true
		}
	}
	return WorldSpec{}, false
}

func (c Config) EntryPoint(worldID, entryID string) (EntryPointSpec, bool) {
	for _, w := range c.Worlds {
		if w.ID != worldID {
			continue
		}
		for _, ep := range w.EntryPoints {
			if ep.ID == entryID {
				return ep, true
			}
		}
		return EntryPointSpec{}, false
	}
	return EntryPointSpec{}, false
}
