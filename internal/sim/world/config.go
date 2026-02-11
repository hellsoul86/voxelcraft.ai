package world

type WorldConfig struct {
	ID                  string
	WorldType           string
	TickRateHz          int
	DayTicks            int
	SeasonLengthTicks   int
	ResetEveryTicks     int
	ResetNoticeTicks    int
	ObsRadius           int
	Height              int
	Seed                int64
	BoundaryR           int
	SwitchCooldownTicks int

	AllowClaims bool
	AllowMine   bool
	AllowPlace  bool
	AllowLaws   bool
	AllowTrade  bool
	AllowBuild  bool

	// Worldgen tuning (pure 2D tilemap).
	BiomeRegionSize                 int
	SpawnClearRadius                int
	OreClusterProbScalePermille     int
	TerrainClusterProbScalePermille int
	SprinkleStonePermille           int
	SprinkleDirtPermille            int
	SprinkleLogPermille             int

	// Starter items granted to newly joined agents.
	// If nil, defaults are applied; if non-nil but empty, new agents get no starter items.
	StarterItems map[string]int

	// Operational parameters. These are included in snapshots for deterministic replay/resume.
	SnapshotEveryTicks int
	DirectorEveryTicks int
	RateLimits         RateLimitConfig

	// Governance.
	LawNoticeTicks int
	LawVoteTicks   int

	// Blueprints.
	BlueprintAutoPullRange int
	BlueprintBlocksPerTick int

	// Claims/laws.
	AccessPassCoreRadius int
	MaintenanceCost      map[string]int

	// Fun-score.
	FunDecayWindowTicks    int
	FunDecayBase           float64
	StructureSurvivalTicks int
}

type RateLimitConfig struct {
	SayWindowTicks        int
	SayMax                int
	MarketSayWindowTicks  int
	MarketSayMax          int
	WhisperWindowTicks    int
	WhisperMax            int
	OfferTradeWindowTicks int
	OfferTradeMax         int
	PostBoardWindowTicks  int
	PostBoardMax          int
}

func (c *WorldConfig) applyDefaults() {
	if c.TickRateHz <= 0 {
		c.TickRateHz = 5
	}
	if c.DayTicks <= 0 {
		c.DayTicks = 6000
	}
	if c.SeasonLengthTicks <= 0 {
		c.SeasonLengthTicks = c.DayTicks * 7
	}
	if c.ResetEveryTicks <= 0 {
		c.ResetEveryTicks = c.SeasonLengthTicks
	}
	if c.SwitchCooldownTicks <= 0 {
		c.SwitchCooldownTicks = 150
	}
	if c.ObsRadius <= 0 {
		c.ObsRadius = 7
	}
	if c.Height <= 0 {
		c.Height = 1
	}
	if c.BoundaryR <= 0 {
		c.BoundaryR = 4000
	}
	if c.BiomeRegionSize <= 0 {
		c.BiomeRegionSize = 64
	}
	if c.SpawnClearRadius <= 0 {
		c.SpawnClearRadius = 6
	}
	if c.OreClusterProbScalePermille <= 0 {
		c.OreClusterProbScalePermille = 1000
	}
	if c.TerrainClusterProbScalePermille <= 0 {
		c.TerrainClusterProbScalePermille = 1000
	}
	if c.SprinkleStonePermille <= 0 {
		c.SprinkleStonePermille = 12
	}
	if c.SprinkleDirtPermille <= 0 {
		c.SprinkleDirtPermille = 4
	}
	if c.SprinkleLogPermille <= 0 {
		c.SprinkleLogPermille = 2
	}
	if c.StarterItems == nil {
		c.StarterItems = map[string]int{
			"PLANK":   20,
			"COAL":    10,
			"STONE":   20,
			"BERRIES": 10,
		}
	}
	if c.SnapshotEveryTicks <= 0 {
		c.SnapshotEveryTicks = 3000
	}
	if c.DirectorEveryTicks <= 0 {
		c.DirectorEveryTicks = 3000
	}
	c.RateLimits.applyDefaults()

	if c.LawNoticeTicks <= 0 {
		c.LawNoticeTicks = 3000
	}
	if c.LawVoteTicks <= 0 {
		c.LawVoteTicks = 3000
	}
	if c.BlueprintAutoPullRange <= 0 {
		c.BlueprintAutoPullRange = 32
	}
	if c.BlueprintBlocksPerTick <= 0 {
		c.BlueprintBlocksPerTick = 2
	}
	if c.AccessPassCoreRadius <= 0 {
		c.AccessPassCoreRadius = 16
	}
	if len(c.MaintenanceCost) == 0 {
		c.MaintenanceCost = map[string]int{
			"IRON_INGOT": 1,
			"COAL":       1,
		}
	}
	if c.FunDecayWindowTicks <= 0 {
		c.FunDecayWindowTicks = 3000
	}
	if c.FunDecayBase <= 0 || c.FunDecayBase > 1.0 {
		c.FunDecayBase = 0.70
	}
	if c.StructureSurvivalTicks <= 0 {
		c.StructureSurvivalTicks = 3000
	}
	// World capability defaults keep legacy single-world behavior.
	if !c.AllowClaims && !c.AllowMine && !c.AllowPlace && !c.AllowLaws && !c.AllowTrade && !c.AllowBuild {
		c.AllowClaims = true
		c.AllowMine = true
		c.AllowPlace = true
		c.AllowLaws = true
		c.AllowTrade = true
		c.AllowBuild = true
	}
}

func (rl *RateLimitConfig) applyDefaults() {
	if rl.SayWindowTicks <= 0 {
		rl.SayWindowTicks = 50
	}
	if rl.SayMax <= 0 {
		rl.SayMax = 5
	}
	if rl.MarketSayWindowTicks <= 0 {
		rl.MarketSayWindowTicks = 50
	}
	if rl.MarketSayMax <= 0 {
		rl.MarketSayMax = 2
	}
	if rl.WhisperWindowTicks <= 0 {
		rl.WhisperWindowTicks = 50
	}
	if rl.WhisperMax <= 0 {
		rl.WhisperMax = 5
	}
	if rl.OfferTradeWindowTicks <= 0 {
		rl.OfferTradeWindowTicks = 50
	}
	if rl.OfferTradeMax <= 0 {
		rl.OfferTradeMax = 3
	}
	if rl.PostBoardWindowTicks <= 0 {
		rl.PostBoardWindowTicks = 600
	}
	if rl.PostBoardMax <= 0 {
		rl.PostBoardMax = 1
	}
}
