package tuning

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"voxelcraft.ai/internal/protocol"
)

type Tuning struct {
	ProtocolVersion string `yaml:"protocol_version"`

	TickRateHz     int   `yaml:"tick_rate_hz"`
	TickDurationMs int   `yaml:"tick_duration_ms"`
	DayTicks       int   `yaml:"day_ticks"`
	SeasonLengthTicks int `yaml:"season_length_ticks"`
	ChunkSize      []int `yaml:"chunk_size"`
	ObsRadius      int   `yaml:"obs_radius"`
	WorldBoundaryR int   `yaml:"world_boundary_r"`

	SnapshotEveryTicks int `yaml:"snapshot_every_ticks"`
	DirectorEveryTicks int `yaml:"director_every_ticks"`

	RateLimits RateLimits `yaml:"rate_limits"`

	// Governance.
	LawNoticeTicks int `yaml:"law_notice_ticks"`
	LawVoteTicks   int `yaml:"law_vote_ticks"`

	// Blueprints.
	BlueprintAutoPullRange int            `yaml:"blueprint_auto_pull_range"`
	BlueprintBlocksPerTick int            `yaml:"blueprint_blocks_per_tick"`
	AccessPassCoreRadius   int            `yaml:"access_pass_core_radius"`
	StructureSurvivalTicks int            `yaml:"structure_survival_ticks"`
	FunDecayWindowTicks    int            `yaml:"fun_decay_window_ticks"`
	FunDecayBase           float64        `yaml:"fun_decay_base"`
	ClaimMaintenanceCost   map[string]int `yaml:"claim_maintenance_cost"`
}

type RateLimits struct {
	SayWindowTicks        int `yaml:"say_window_ticks"`
	SayMax                int `yaml:"say_max"`
	WhisperWindowTicks    int `yaml:"whisper_window_ticks"`
	WhisperMax            int `yaml:"whisper_max"`
	OfferTradeWindowTicks int `yaml:"offer_trade_window_ticks"`
	OfferTradeMax         int `yaml:"offer_trade_max"`
	PostBoardWindowTicks  int `yaml:"post_board_window_ticks"`
	PostBoardMax          int `yaml:"post_board_max"`
}

func Load(path string) (Tuning, error) {
	t := Defaults()
	raw, err := os.ReadFile(path)
	if err != nil {
		return t, err
	}
	if err := yaml.Unmarshal(raw, &t); err != nil {
		return t, fmt.Errorf("tuning.yaml: %w", err)
	}
	if err := t.Validate(); err != nil {
		return t, fmt.Errorf("tuning.yaml: %w", err)
	}
	return t, nil
}

func Defaults() Tuning {
	return Tuning{
		ProtocolVersion: protocol.Version,

		TickRateHz:     5,
		TickDurationMs: 200,
		DayTicks:       6000,
		SeasonLengthTicks: 6000 * 7, // 7 in-game days
		ChunkSize:      []int{16, 16, 64},
		ObsRadius:      7,
		WorldBoundaryR: 4000,

		SnapshotEveryTicks: 3000,
		DirectorEveryTicks: 3000,

		RateLimits: RateLimits{
			SayWindowTicks:        50,
			SayMax:                5,
			WhisperWindowTicks:    50,
			WhisperMax:            5,
			OfferTradeWindowTicks: 50,
			OfferTradeMax:         3,
			PostBoardWindowTicks:  600,
			PostBoardMax:          1,
		},

		LawNoticeTicks: 3000,
		LawVoteTicks:   3000,

		BlueprintAutoPullRange: 32,
		BlueprintBlocksPerTick: 2,

		AccessPassCoreRadius:   16,
		StructureSurvivalTicks: 3000,

		FunDecayWindowTicks: 3000,
		FunDecayBase:        0.70,

		ClaimMaintenanceCost: map[string]int{
			"IRON_INGOT": 1,
			"COAL":       1,
		},
	}
}

func (t Tuning) Validate() error {
	if t.ProtocolVersion != protocol.Version {
		return fmt.Errorf("protocol_version must be %q (got %q)", protocol.Version, t.ProtocolVersion)
	}

	if t.TickRateHz <= 0 {
		return fmt.Errorf("tick_rate_hz must be > 0 (got %d)", t.TickRateHz)
	}
	if t.TickDurationMs <= 0 {
		return fmt.Errorf("tick_duration_ms must be > 0 (got %d)", t.TickDurationMs)
	}
	// Keep tick_rate_hz and tick_duration_ms consistent to avoid accidental drift.
	if 1000%t.TickRateHz != 0 {
		return fmt.Errorf("tick_rate_hz must divide 1000 cleanly (got %d)", t.TickRateHz)
	}
	wantMs := 1000 / t.TickRateHz
	if t.TickDurationMs != wantMs {
		return fmt.Errorf("tick_duration_ms mismatch: want %d for tick_rate_hz=%d (got %d)", wantMs, t.TickRateHz, t.TickDurationMs)
	}

	if t.DayTicks <= 0 {
		return fmt.Errorf("day_ticks must be > 0 (got %d)", t.DayTicks)
	}
	if t.SeasonLengthTicks <= 0 {
		return fmt.Errorf("season_length_ticks must be > 0 (got %d)", t.SeasonLengthTicks)
	}
	if t.SeasonLengthTicks%t.DayTicks != 0 {
		return fmt.Errorf("season_length_ticks must be a multiple of day_ticks (season_length_ticks=%d day_ticks=%d)", t.SeasonLengthTicks, t.DayTicks)
	}

	if len(t.ChunkSize) != 3 {
		return fmt.Errorf("chunk_size must be [x,z,height] (got %v)", t.ChunkSize)
	}
	if t.ChunkSize[0] != 16 || t.ChunkSize[1] != 16 {
		return fmt.Errorf("chunk_size x/z must be 16 (got %v)", t.ChunkSize)
	}
	if t.ChunkSize[2] <= 0 {
		return fmt.Errorf("chunk_size height must be > 0 (got %d)", t.ChunkSize[2])
	}

	if t.ObsRadius <= 0 || t.ObsRadius > 32 {
		return fmt.Errorf("obs_radius must be in 1..32 (got %d)", t.ObsRadius)
	}
	if t.WorldBoundaryR <= 0 {
		return fmt.Errorf("world_boundary_r must be > 0 (got %d)", t.WorldBoundaryR)
	}
	if t.SnapshotEveryTicks <= 0 {
		return fmt.Errorf("snapshot_every_ticks must be > 0 (got %d)", t.SnapshotEveryTicks)
	}
	if t.DirectorEveryTicks <= 0 {
		return fmt.Errorf("director_every_ticks must be > 0 (got %d)", t.DirectorEveryTicks)
	}

	if err := validateWindowMax("rate_limits.say", t.RateLimits.SayWindowTicks, t.RateLimits.SayMax); err != nil {
		return err
	}
	if err := validateWindowMax("rate_limits.whisper", t.RateLimits.WhisperWindowTicks, t.RateLimits.WhisperMax); err != nil {
		return err
	}
	if err := validateWindowMax("rate_limits.offer_trade", t.RateLimits.OfferTradeWindowTicks, t.RateLimits.OfferTradeMax); err != nil {
		return err
	}
	if err := validateWindowMax("rate_limits.post_board", t.RateLimits.PostBoardWindowTicks, t.RateLimits.PostBoardMax); err != nil {
		return err
	}

	if t.LawNoticeTicks <= 0 {
		return fmt.Errorf("law_notice_ticks must be > 0 (got %d)", t.LawNoticeTicks)
	}
	if t.LawVoteTicks <= 0 {
		return fmt.Errorf("law_vote_ticks must be > 0 (got %d)", t.LawVoteTicks)
	}
	if t.BlueprintAutoPullRange <= 0 {
		return fmt.Errorf("blueprint_auto_pull_range must be > 0 (got %d)", t.BlueprintAutoPullRange)
	}
	if t.BlueprintBlocksPerTick <= 0 {
		return fmt.Errorf("blueprint_blocks_per_tick must be > 0 (got %d)", t.BlueprintBlocksPerTick)
	}
	if t.AccessPassCoreRadius <= 0 {
		return fmt.Errorf("access_pass_core_radius must be > 0 (got %d)", t.AccessPassCoreRadius)
	}
	if t.StructureSurvivalTicks <= 0 {
		return fmt.Errorf("structure_survival_ticks must be > 0 (got %d)", t.StructureSurvivalTicks)
	}
	if t.FunDecayWindowTicks <= 0 {
		return fmt.Errorf("fun_decay_window_ticks must be > 0 (got %d)", t.FunDecayWindowTicks)
	}
	if t.FunDecayBase <= 0 || t.FunDecayBase > 1.0 {
		return fmt.Errorf("fun_decay_base must be in (0,1] (got %v)", t.FunDecayBase)
	}
	if len(t.ClaimMaintenanceCost) == 0 {
		return fmt.Errorf("claim_maintenance_cost must not be empty")
	}
	for item, n := range t.ClaimMaintenanceCost {
		if item == "" {
			return fmt.Errorf("claim_maintenance_cost contains empty item id")
		}
		if n <= 0 {
			return fmt.Errorf("claim_maintenance_cost[%s] must be > 0 (got %d)", item, n)
		}
	}

	return nil
}

func validateWindowMax(name string, windowTicks, max int) error {
	if windowTicks <= 0 {
		return fmt.Errorf("%s window_ticks must be > 0 (got %d)", name, windowTicks)
	}
	if max <= 0 {
		return fmt.Errorf("%s max must be > 0 (got %d)", name, max)
	}
	return nil
}
