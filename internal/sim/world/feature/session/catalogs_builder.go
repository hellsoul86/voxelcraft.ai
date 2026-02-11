package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"voxelcraft.ai/internal/protocol"
	"voxelcraft.ai/internal/sim/catalogs"
)

type TuningCatalog struct {
	SnapshotEveryTicks int `json:"snapshot_every_ticks"`
	DirectorEveryTicks int `json:"director_every_ticks"`
	SeasonLengthTicks  int `json:"season_length_ticks"`

	RateLimits TuningRateLimits `json:"rate_limits"`

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

type TuningRateLimits struct {
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

type LawTemplatesCatalogData struct {
	Templates []catalogs.LawTemplate `json:"templates"`
}

type TuningInput struct {
	SnapshotEveryTicks int
	DirectorEveryTicks int
	SeasonLengthTicks  int

	RateLimits TuningRateLimits

	LawNoticeTicks int
	LawVoteTicks   int

	BlueprintAutoPullRange int
	BlueprintBlocksPerTick int

	AccessPassCoreRadius int
	MaintenanceCost      map[string]int

	FunDecayWindowTicks    int
	FunDecayBase           float64
	StructureSurvivalTicks int
}

func TuningCatalogMsg(in TuningInput) protocol.CatalogMsg {
	cost := make([]protocol.ItemStack, 0, len(in.MaintenanceCost))
	for item, n := range in.MaintenanceCost {
		if item == "" || n <= 0 {
			continue
		}
		cost = append(cost, protocol.ItemStack{Item: item, Count: n})
	}
	sort.Slice(cost, func(i, j int) bool { return cost[i].Item < cost[j].Item })

	tc := TuningCatalog{
		SnapshotEveryTicks: in.SnapshotEveryTicks,
		DirectorEveryTicks: in.DirectorEveryTicks,
		SeasonLengthTicks:  in.SeasonLengthTicks,
		RateLimits:         in.RateLimits,
		LawNoticeTicks:     in.LawNoticeTicks,
		LawVoteTicks:       in.LawVoteTicks,

		BlueprintAutoPullRange: in.BlueprintAutoPullRange,
		BlueprintBlocksPerTick: in.BlueprintBlocksPerTick,

		AccessPassCoreRadius:   in.AccessPassCoreRadius,
		ClaimMaintenanceCost:   cost,
		FunDecayWindowTicks:    in.FunDecayWindowTicks,
		FunDecayBase:           in.FunDecayBase,
		StructureSurvivalTicks: in.StructureSurvivalTicks,
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

func RecipesCatalogMsg(digest string, byID map[string]catalogs.RecipeDef) protocol.CatalogMsg {
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	defs := make([]catalogs.RecipeDef, 0, len(ids))
	for _, id := range ids {
		defs = append(defs, byID[id])
	}
	return protocol.CatalogMsg{
		Type:            protocol.TypeCatalog,
		ProtocolVersion: protocol.Version,
		Name:            "recipes",
		Digest:          digest,
		Part:            1,
		TotalParts:      1,
		Data:            defs,
	}
}

func BlueprintsCatalogMsg(digest string, byID map[string]catalogs.BlueprintDef) protocol.CatalogMsg {
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	defs := make([]catalogs.BlueprintDef, 0, len(ids))
	for _, id := range ids {
		defs = append(defs, byID[id])
	}
	return protocol.CatalogMsg{
		Type:            protocol.TypeCatalog,
		ProtocolVersion: protocol.Version,
		Name:            "blueprints",
		Digest:          digest,
		Part:            1,
		TotalParts:      1,
		Data:            defs,
	}
}

func LawTemplatesCatalogMsg(digest string, templates []catalogs.LawTemplate) protocol.CatalogMsg {
	tpls := make([]catalogs.LawTemplate, 0, len(templates))
	tpls = append(tpls, templates...)
	sort.Slice(tpls, func(i, j int) bool { return tpls[i].ID < tpls[j].ID })
	return protocol.CatalogMsg{
		Type:            protocol.TypeCatalog,
		ProtocolVersion: protocol.Version,
		Name:            "law_templates",
		Digest:          digest,
		Part:            1,
		TotalParts:      1,
		Data:            LawTemplatesCatalogData{Templates: tpls},
	}
}

func EventsCatalogMsg(digest string, byID map[string]catalogs.EventTemplate) protocol.CatalogMsg {
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	defs := make([]catalogs.EventTemplate, 0, len(ids))
	for _, id := range ids {
		defs = append(defs, byID[id])
	}
	return protocol.CatalogMsg{
		Type:            protocol.TypeCatalog,
		ProtocolVersion: protocol.Version,
		Name:            "events",
		Digest:          digest,
		Part:            1,
		TotalParts:      1,
		Data:            defs,
	}
}
