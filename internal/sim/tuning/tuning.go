package tuning

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Tuning struct {
	ProtocolVersion string `yaml:"protocol_version"`

	TickRateHz       int `yaml:"tick_rate_hz"`
	TickDurationMs   int `yaml:"tick_duration_ms"`
	DayTicks         int `yaml:"day_ticks"`
	ChunkSize        []int `yaml:"chunk_size"`
	ObsRadius        int `yaml:"obs_radius"`
	WorldBoundaryR   int `yaml:"world_boundary_r"`
	SnapshotEveryTicks int `yaml:"snapshot_every_ticks"`

	RateLimits RateLimits `yaml:"rate_limits"`
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
	var t Tuning
	raw, err := os.ReadFile(path)
	if err != nil {
		return t, err
	}
	if err := yaml.Unmarshal(raw, &t); err != nil {
		return t, fmt.Errorf("tuning.yaml: %w", err)
	}
	return t, nil
}

