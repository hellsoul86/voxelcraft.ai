package world

import (
	"fmt"

	"voxelcraft.ai/internal/persistence/snapshot"
	snapshotfeaturepkg "voxelcraft.ai/internal/sim/world/feature/persistence/snapshot"
	"voxelcraft.ai/internal/sim/world/io/snapshotcodec"
	storepkg "voxelcraft.ai/internal/sim/world/terrain/store"
)

func (w *World) exportChunkSnapshots() []snapshot.ChunkV1 {
	keys := w.chunks.LoadedChunkKeys()
	return storepkg.ExportLoadedChunks(w.chunks.chunks, keys)
}

func (w *World) exportSnapshot(nowTick uint64) snapshot.SnapshotV1 {
	// Snapshot must be called from the world loop goroutine.
	chunks := w.exportChunkSnapshots()

	return snapshot.SnapshotV1{
		Header: snapshot.Header{
			Version: 1,
			WorldID: w.cfg.ID,
			Tick:    nowTick,
		},
		Seed:                            w.cfg.Seed,
		TickRate:                        w.cfg.TickRateHz,
		DayTicks:                        w.cfg.DayTicks,
		SeasonLengthTicks:               w.cfg.SeasonLengthTicks,
		ObsRadius:                       w.cfg.ObsRadius,
		Height:                          w.cfg.Height,
		BoundaryR:                       w.cfg.BoundaryR,
		BiomeRegionSize:                 w.cfg.BiomeRegionSize,
		SpawnClearRadius:                w.cfg.SpawnClearRadius,
		OreClusterProbScalePermille:     w.cfg.OreClusterProbScalePermille,
		TerrainClusterProbScalePermille: w.cfg.TerrainClusterProbScalePermille,
		SprinkleStonePermille:           w.cfg.SprinkleStonePermille,
		SprinkleDirtPermille:            w.cfg.SprinkleDirtPermille,
		SprinkleLogPermille:             w.cfg.SprinkleLogPermille,
		StarterItems:                    snapshotcodec.PositiveMap(w.cfg.StarterItems),
		SnapshotEveryTicks:              w.cfg.SnapshotEveryTicks,
		DirectorEveryTicks:              w.cfg.DirectorEveryTicks,
		RateLimits: snapshot.RateLimitsV1{
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
		LawNoticeTicks:         w.cfg.LawNoticeTicks,
		LawVoteTicks:           w.cfg.LawVoteTicks,
		BlueprintAutoPullRange: w.cfg.BlueprintAutoPullRange,
		BlueprintBlocksPerTick: w.cfg.BlueprintBlocksPerTick,
		AccessPassCoreRadius:   w.cfg.AccessPassCoreRadius,
		MaintenanceCost:        snapshotcodec.PositiveMap(w.cfg.MaintenanceCost),
		FunDecayWindowTicks:    w.cfg.FunDecayWindowTicks,
		FunDecayBase:           w.cfg.FunDecayBase,
		StructureSurvivalTicks: w.cfg.StructureSurvivalTicks,
		Weather:                w.weather,
		WeatherUntilTick:       w.weatherUntilTick,
		ActiveEventID:          w.activeEventID,
		ActiveEventStart:       w.activeEventStart,
		ActiveEventEnds:        w.activeEventEnds,
		ActiveEventCenter:      w.activeEventCenter.ToArray(),
		ActiveEventRadius:      w.activeEventRadius,
		Chunks:                 chunks,
		Agents:                 snapshotfeaturepkg.ExportAgents(nowTick, w.agents),
		Claims:                 snapshotfeaturepkg.ExportClaims(w.claims),
		Containers:             snapshotfeaturepkg.ExportContainers(w.containers),
		Items:                  snapshotfeaturepkg.ExportItems(nowTick, w.items),
		Signs:                  snapshotfeaturepkg.ExportSigns(w.signs),
		Conveyors:              snapshotfeaturepkg.ExportConveyors(w.conveyors),
		Switches:               snapshotfeaturepkg.ExportSwitches(w.switches),
		Trades:                 snapshotfeaturepkg.ExportTrades(w.trades),
		Boards:                 snapshotfeaturepkg.ExportBoards(w.boards),
		Contracts:              snapshotfeaturepkg.ExportContracts(w.contracts),
		Laws:                   snapshotfeaturepkg.ExportLaws(w.laws),
		Orgs:                   snapshotfeaturepkg.ExportOrgs(w.orgs),
		Structures:             snapshotfeaturepkg.ExportStructures(w.structures),
		Stats:                  snapshotfeaturepkg.ExportStats(w.stats),
		Counters: snapshot.CountersV1{
			NextAgent:    w.nextAgentNum.Load(),
			NextTask:     w.nextTaskNum.Load(),
			NextLand:     w.nextLandNum.Load(),
			NextTrade:    w.nextTradeNum.Load(),
			NextPost:     w.nextPostNum.Load(),
			NextContract: w.nextContractNum.Load(),
			NextLaw:      w.nextLawNum.Load(),
			NextOrg:      w.nextOrgNum.Load(),
			NextItem:     w.nextItemNum.Load(),
		},
	}
}

func (w *World) importSnapshotV1(s snapshot.SnapshotV1) error {
	if err := w.validateSnapshotImport(s); err != nil {
		return err
	}
	w.applySnapshotConfig(s)
	w.applySnapshotRuntimeState(s)

	// Rebuild chunks.
	gen := w.chunks.gen
	gen.Seed = w.cfg.Seed
	gen.BoundaryR = w.cfg.BoundaryR
	gen.BiomeRegionSize = w.cfg.BiomeRegionSize
	gen.SpawnClearRadius = w.cfg.SpawnClearRadius
	gen.OreClusterProbScalePermille = w.cfg.OreClusterProbScalePermille
	gen.TerrainClusterProbScalePermille = w.cfg.TerrainClusterProbScalePermille
	gen.SprinkleStonePermille = w.cfg.SprinkleStonePermille
	gen.SprinkleDirtPermille = w.cfg.SprinkleDirtPermille
	gen.SprinkleLogPermille = w.cfg.SprinkleLogPermille

	if err := w.importChunkSnapshots(gen, s.Chunks); err != nil {
		return err
	}

	agents, maxAgent, maxTask := snapshotfeaturepkg.ImportAgents(s)
	w.agents = agents
	w.clients = map[string]*clientState{}
	w.nextAgentNum.Store(snapshotfeaturepkg.MaxU64(maxAgent, s.Counters.NextAgent))
	w.nextTaskNum.Store(snapshotfeaturepkg.MaxU64(maxTask, s.Counters.NextTask))

	claims, maxLand := snapshotfeaturepkg.ImportClaims(s)
	w.claims = claims
	w.nextLandNum.Store(snapshotfeaturepkg.MaxU64(maxLand, s.Counters.NextLand))

	w.containers = snapshotfeaturepkg.ImportContainers(s)

	items, itemsAt, maxItem := snapshotfeaturepkg.ImportItems(s)
	w.items = items
	w.itemsAt = itemsAt
	w.nextItemNum.Store(snapshotfeaturepkg.MaxU64(maxItem, s.Counters.NextItem))

	blockNameAt := func(pos Vec3i) string {
		return w.blockName(w.chunks.GetBlock(pos))
	}
	w.signs = snapshotfeaturepkg.ImportSigns(s, blockNameAt)
	w.conveyors = snapshotfeaturepkg.ImportConveyors(s, blockNameAt)
	w.switches = snapshotfeaturepkg.ImportSwitches(s, blockNameAt)

	trades, maxTrade := snapshotfeaturepkg.ImportTrades(s)
	w.trades = trades
	w.nextTradeNum.Store(snapshotfeaturepkg.MaxU64(maxTrade, s.Counters.NextTrade))

	boards, maxPost := snapshotfeaturepkg.ImportBoards(s)
	w.boards = boards
	w.nextPostNum.Store(snapshotfeaturepkg.MaxU64(maxPost, s.Counters.NextPost))

	contracts, maxContract := snapshotfeaturepkg.ImportContracts(s)
	w.contracts = contracts
	w.nextContractNum.Store(snapshotfeaturepkg.MaxU64(maxContract, s.Counters.NextContract))

	laws, maxLaw := snapshotfeaturepkg.ImportLaws(s)
	w.laws = laws
	w.nextLawNum.Store(snapshotfeaturepkg.MaxU64(maxLaw, s.Counters.NextLaw))

	orgs, maxOrg := snapshotfeaturepkg.ImportOrgs(s)
	w.orgs = orgs
	w.nextOrgNum.Store(snapshotfeaturepkg.MaxU64(maxOrg, s.Counters.NextOrg))

	w.structures = snapshotfeaturepkg.ImportStructures(s)
	w.stats = snapshotfeaturepkg.ImportStats(s)

	// Resume on the next tick.
	w.tick.Store(s.Header.Tick + 1)
	return nil
}

func (w *World) importChunkSnapshots(gen WorldGen, chunks []snapshot.ChunkV1) error {
	inner, err := storepkg.ImportChunks(gen, chunks)
	if err != nil {
		return err
	}
	w.chunks = &ChunkStore{
		inner:  inner,
		gen:    inner.Gen,
		chunks: inner.Chunks,
	}
	return nil
}

func (w *World) validateSnapshotImport(s snapshot.SnapshotV1) error {
	if s.Header.Version != 1 {
		return fmt.Errorf("unsupported snapshot version: %d", s.Header.Version)
	}
	if w.cfg.Seed != s.Seed {
		return fmt.Errorf("snapshot seed mismatch: cfg=%d snap=%d", w.cfg.Seed, s.Seed)
	}
	if w.cfg.Height != s.Height {
		return fmt.Errorf("snapshot height mismatch: cfg=%d snap=%d", w.cfg.Height, s.Height)
	}
	if s.Height != 1 {
		return fmt.Errorf("unsupported snapshot height for 2D world: height=%d", s.Height)
	}
	if w.cfg.DayTicks != s.DayTicks {
		return fmt.Errorf("snapshot day_ticks mismatch: cfg=%d snap=%d", w.cfg.DayTicks, s.DayTicks)
	}
	if w.cfg.ObsRadius != s.ObsRadius {
		return fmt.Errorf("snapshot obs_radius mismatch: cfg=%d snap=%d", w.cfg.ObsRadius, s.ObsRadius)
	}
	if w.cfg.BoundaryR != s.BoundaryR {
		return fmt.Errorf("snapshot boundary_r mismatch: cfg=%d snap=%d", w.cfg.BoundaryR, s.BoundaryR)
	}
	return nil
}

func (w *World) applySnapshotConfig(s snapshot.SnapshotV1) {
	patch := snapshotfeaturepkg.BuildConfigPatch(s)
	if patch.BiomeRegionSize > 0 {
		w.cfg.BiomeRegionSize = patch.BiomeRegionSize
	}
	if patch.SpawnClearRadius > 0 {
		w.cfg.SpawnClearRadius = patch.SpawnClearRadius
	}
	if patch.OreClusterProbScalePermille > 0 {
		w.cfg.OreClusterProbScalePermille = patch.OreClusterProbScalePermille
	}
	if patch.TerrainClusterProbScalePermille > 0 {
		w.cfg.TerrainClusterProbScalePermille = patch.TerrainClusterProbScalePermille
	}
	if patch.SprinkleStonePermille > 0 {
		w.cfg.SprinkleStonePermille = patch.SprinkleStonePermille
	}
	if patch.SprinkleDirtPermille > 0 {
		w.cfg.SprinkleDirtPermille = patch.SprinkleDirtPermille
	}
	if patch.SprinkleLogPermille > 0 {
		w.cfg.SprinkleLogPermille = patch.SprinkleLogPermille
	}

	if patch.HasStarterItems {
		w.cfg.StarterItems = patch.StarterItems
	}

	if patch.SnapshotEveryTicks > 0 {
		w.cfg.SnapshotEveryTicks = patch.SnapshotEveryTicks
	}
	if patch.DirectorEveryTicks > 0 {
		w.cfg.DirectorEveryTicks = patch.DirectorEveryTicks
	}
	if s.SeasonLengthTicks > 0 {
		w.cfg.SeasonLengthTicks = s.SeasonLengthTicks
	}
	if patch.HasRateLimits {
		w.cfg.RateLimits = RateLimitConfig{
			SayWindowTicks:        patch.RateLimits.SayWindowTicks,
			SayMax:                patch.RateLimits.SayMax,
			MarketSayWindowTicks:  patch.RateLimits.MarketSayWindowTicks,
			MarketSayMax:          patch.RateLimits.MarketSayMax,
			WhisperWindowTicks:    patch.RateLimits.WhisperWindowTicks,
			WhisperMax:            patch.RateLimits.WhisperMax,
			OfferTradeWindowTicks: patch.RateLimits.OfferTradeWindowTicks,
			OfferTradeMax:         patch.RateLimits.OfferTradeMax,
			PostBoardWindowTicks:  patch.RateLimits.PostBoardWindowTicks,
			PostBoardMax:          patch.RateLimits.PostBoardMax,
		}
	}
	if patch.LawNoticeTicks > 0 {
		w.cfg.LawNoticeTicks = patch.LawNoticeTicks
	}
	if patch.LawVoteTicks > 0 {
		w.cfg.LawVoteTicks = patch.LawVoteTicks
	}
	if patch.BlueprintAutoPullRange > 0 {
		w.cfg.BlueprintAutoPullRange = patch.BlueprintAutoPullRange
	}
	if patch.BlueprintBlocksPerTick > 0 {
		w.cfg.BlueprintBlocksPerTick = patch.BlueprintBlocksPerTick
	}
	if patch.AccessPassCoreRadius > 0 {
		w.cfg.AccessPassCoreRadius = patch.AccessPassCoreRadius
	}
	if patch.HasMaintenanceCost {
		w.cfg.MaintenanceCost = patch.MaintenanceCost
		if len(w.cfg.MaintenanceCost) == 0 {
			w.cfg.MaintenanceCost = nil
		}
	}
	if patch.FunDecayWindowTicks > 0 {
		w.cfg.FunDecayWindowTicks = patch.FunDecayWindowTicks
	}
	if patch.FunDecayBase > 0 {
		w.cfg.FunDecayBase = patch.FunDecayBase
	}
	if patch.StructureSurvivalTicks > 0 {
		w.cfg.StructureSurvivalTicks = patch.StructureSurvivalTicks
	}
	w.cfg.applyDefaults()
}

func (w *World) applySnapshotRuntimeState(s snapshot.SnapshotV1) {
	patch := snapshotfeaturepkg.BuildRuntimePatch(s)
	w.weather = patch.Weather
	w.weatherUntilTick = patch.WeatherUntilTick
	w.activeEventID = patch.ActiveEventID
	w.activeEventStart = patch.ActiveEventStart
	w.activeEventEnds = patch.ActiveEventEnds
	w.activeEventCenter = Vec3i{
		X: patch.ActiveEventCenter[0],
		Y: patch.ActiveEventCenter[1],
		Z: patch.ActiveEventCenter[2],
	}
	w.activeEventRadius = patch.ActiveEventRadius
}
