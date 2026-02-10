package world

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"voxelcraft.ai/internal/protocol"
)

type tuningCatalog struct {
	SnapshotEveryTicks int `json:"snapshot_every_ticks"`
	DirectorEveryTicks int `json:"director_every_ticks"`
	SeasonLengthTicks  int `json:"season_length_ticks"`

	RateLimits tuningRateLimits `json:"rate_limits"`

	LawNoticeTicks int `json:"law_notice_ticks"`
	LawVoteTicks   int `json:"law_vote_ticks"`

	BlueprintAutoPullRange int `json:"blueprint_auto_pull_range"`
	BlueprintBlocksPerTick int `json:"blueprint_blocks_per_tick"`

	AccessPassCoreRadius int                  `json:"access_pass_core_radius"`
	ClaimMaintenanceCost []protocol.ItemStack `json:"claim_maintenance_cost"`

	FunDecayWindowTicks    int     `json:"fun_decay_window_ticks"`
	FunDecayBase           float64 `json:"fun_decay_base"`
	StructureSurvivalTicks int     `json:"structure_survival_ticks"`
}

type tuningRateLimits struct {
	SayWindowTicks        int `json:"say_window_ticks"`
	SayMax                int `json:"say_max"`
	MarketSayWindowTicks  int `json:"market_say_window_ticks"`
	MarketSayMax          int `json:"market_say_max"`
	WhisperWindowTicks    int `json:"whisper_window_ticks"`
	WhisperMax            int `json:"whisper_max"`
	OfferTradeWindowTicks int `json:"offer_trade_window_ticks"`
	OfferTradeMax         int `json:"offer_trade_max"`
	PostBoardWindowTicks  int `json:"post_board_window_ticks"`
	PostBoardMax          int `json:"post_board_max"`
}

func (w *World) tuningCatalogMsg() protocol.CatalogMsg {
	cost := make([]protocol.ItemStack, 0, len(w.cfg.MaintenanceCost))
	for item, n := range w.cfg.MaintenanceCost {
		if item == "" || n <= 0 {
			continue
		}
		cost = append(cost, protocol.ItemStack{Item: item, Count: n})
	}
	sort.Slice(cost, func(i, j int) bool { return cost[i].Item < cost[j].Item })

	tc := tuningCatalog{
		SnapshotEveryTicks: w.cfg.SnapshotEveryTicks,
		DirectorEveryTicks: w.cfg.DirectorEveryTicks,
		SeasonLengthTicks:  w.cfg.SeasonLengthTicks,
		RateLimits: tuningRateLimits{
			SayWindowTicks:        w.cfg.RateLimits.SayWindowTicks,
			SayMax:                w.cfg.RateLimits.SayMax,
			MarketSayWindowTicks:  w.cfg.RateLimits.MarketSayWindowTicks,
			MarketSayMax:          w.cfg.RateLimits.MarketSayMax,
			WhisperWindowTicks:    w.cfg.RateLimits.WhisperWindowTicks,
			WhisperMax:            w.cfg.RateLimits.WhisperMax,
			OfferTradeWindowTicks: w.cfg.RateLimits.OfferTradeWindowTicks,
			OfferTradeMax:         w.cfg.RateLimits.OfferTradeMax,
			PostBoardWindowTicks:  w.cfg.RateLimits.PostBoardWindowTicks,
			PostBoardMax:          w.cfg.RateLimits.PostBoardMax,
		},
		LawNoticeTicks: w.cfg.LawNoticeTicks,
		LawVoteTicks:   w.cfg.LawVoteTicks,

		BlueprintAutoPullRange: w.cfg.BlueprintAutoPullRange,
		BlueprintBlocksPerTick: w.cfg.BlueprintBlocksPerTick,

		AccessPassCoreRadius:   w.cfg.AccessPassCoreRadius,
		ClaimMaintenanceCost:   cost,
		FunDecayWindowTicks:    w.cfg.FunDecayWindowTicks,
		FunDecayBase:           w.cfg.FunDecayBase,
		StructureSurvivalTicks: w.cfg.StructureSurvivalTicks,
	}

	b, _ := json.Marshal(tc)
	sum := sha256.Sum256(b)
	digest := hex.EncodeToString(sum[:])

	return protocol.CatalogMsg{
		Type:            protocol.TypeCatalog,
		ProtocolVersion: protocol.Version,
		Name:            "tuning",
		Digest:          digest,
		Part:            1,
		TotalParts:      1,
		Data:            tc,
	}
}
