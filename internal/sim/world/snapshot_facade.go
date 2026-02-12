package world

import (
	"fmt"
	"strings"

	"voxelcraft.ai/internal/persistence/snapshot"
	snapshotfeaturepkg "voxelcraft.ai/internal/sim/world/feature/persistence/snapshot"
	"voxelcraft.ai/internal/sim/world/io/snapshotcodec"
)

func (w *World) exportChunkSnapshots() []snapshot.ChunkV1 {
	keys := w.chunks.LoadedChunkKeys()
	chunks := make([]snapshot.ChunkV1, 0, len(keys))
	for _, k := range keys {
		ch := w.chunks.chunks[k]
		blocks := make([]uint16, len(ch.Blocks))
		copy(blocks, ch.Blocks)
		chunks = append(chunks, snapshot.ChunkV1{
			CX:     k.CX,
			CZ:     k.CZ,
			Height: 1,
			Blocks: blocks,
		})
	}
	return chunks
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
	w.nextAgentNum.Store(maxU64(maxAgent, s.Counters.NextAgent))
	w.nextTaskNum.Store(maxU64(maxTask, s.Counters.NextTask))

	claims, maxLand := snapshotfeaturepkg.ImportClaims(s)
	w.claims = claims
	w.nextLandNum.Store(maxU64(maxLand, s.Counters.NextLand))

	w.containers = snapshotfeaturepkg.ImportContainers(s)

	items, itemsAt, maxItem := snapshotfeaturepkg.ImportItems(s)
	w.items = items
	w.itemsAt = itemsAt
	w.nextItemNum.Store(maxU64(maxItem, s.Counters.NextItem))

	blockNameAt := func(pos Vec3i) string {
		return w.blockName(w.chunks.GetBlock(pos))
	}
	w.signs = snapshotfeaturepkg.ImportSigns(s, blockNameAt)
	w.conveyors = snapshotfeaturepkg.ImportConveyors(s, blockNameAt)
	w.switches = snapshotfeaturepkg.ImportSwitches(s, blockNameAt)

	trades, maxTrade := snapshotfeaturepkg.ImportTrades(s)
	w.trades = trades
	w.nextTradeNum.Store(maxU64(maxTrade, s.Counters.NextTrade))

	boards, maxPost := snapshotfeaturepkg.ImportBoards(s)
	w.boards = boards
	w.nextPostNum.Store(maxU64(maxPost, s.Counters.NextPost))

	contracts, maxContract := snapshotfeaturepkg.ImportContracts(s)
	w.contracts = contracts
	w.nextContractNum.Store(maxU64(maxContract, s.Counters.NextContract))

	laws, maxLaw := snapshotfeaturepkg.ImportLaws(s)
	w.laws = laws
	w.nextLawNum.Store(maxU64(maxLaw, s.Counters.NextLaw))

	orgs, maxOrg := snapshotfeaturepkg.ImportOrgs(s)
	w.orgs = orgs
	w.nextOrgNum.Store(maxU64(maxOrg, s.Counters.NextOrg))

	w.structures = snapshotfeaturepkg.ImportStructures(s)
	w.stats = snapshotfeaturepkg.ImportStats(s)

	// Resume on the next tick.
	w.tick.Store(s.Header.Tick + 1)
	return nil
}

func maxU64(a, b uint64) uint64 {
	if a >= b {
		return a
	}
	return b
}

func (w *World) importChunkSnapshots(gen WorldGen, chunks []snapshot.ChunkV1) error {
	store := NewChunkStore(gen)
	for _, ch := range chunks {
		if ch.Height != 1 {
			return fmt.Errorf("snapshot chunk height mismatch: got %d want 1", ch.Height)
		}
		if len(ch.Blocks) != 16*16 {
			return fmt.Errorf("snapshot chunk blocks length mismatch: got %d want %d", len(ch.Blocks), 16*16)
		}
		k := ChunkKey{CX: ch.CX, CZ: ch.CZ}
		blocks := make([]uint16, len(ch.Blocks))
		copy(blocks, ch.Blocks)
		c := &Chunk{
			CX:     ch.CX,
			CZ:     ch.CZ,
			Blocks: blocks,
		}
		_ = c.Digest()
		store.chunks[k] = c
	}
	w.chunks = store
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
	if s.BiomeRegionSize > 0 {
		w.cfg.BiomeRegionSize = s.BiomeRegionSize
	}
	if s.SpawnClearRadius > 0 {
		w.cfg.SpawnClearRadius = s.SpawnClearRadius
	}
	if s.OreClusterProbScalePermille > 0 {
		w.cfg.OreClusterProbScalePermille = s.OreClusterProbScalePermille
	}
	if s.TerrainClusterProbScalePermille > 0 {
		w.cfg.TerrainClusterProbScalePermille = s.TerrainClusterProbScalePermille
	}
	if s.SprinkleStonePermille > 0 {
		w.cfg.SprinkleStonePermille = s.SprinkleStonePermille
	}
	if s.SprinkleDirtPermille > 0 {
		w.cfg.SprinkleDirtPermille = s.SprinkleDirtPermille
	}
	if s.SprinkleLogPermille > 0 {
		w.cfg.SprinkleLogPermille = s.SprinkleLogPermille
	}

	if s.StarterItems != nil {
		w.cfg.StarterItems = map[string]int{}
		for item, n := range s.StarterItems {
			if strings.TrimSpace(item) == "" || n <= 0 {
				continue
			}
			w.cfg.StarterItems[item] = n
		}
	}

	if s.SnapshotEveryTicks > 0 {
		w.cfg.SnapshotEveryTicks = s.SnapshotEveryTicks
	}
	if s.DirectorEveryTicks > 0 {
		w.cfg.DirectorEveryTicks = s.DirectorEveryTicks
	}
	if s.SeasonLengthTicks > 0 {
		w.cfg.SeasonLengthTicks = s.SeasonLengthTicks
	}
	if s.RateLimits.SayWindowTicks > 0 ||
		s.RateLimits.SayMax > 0 ||
		s.RateLimits.MarketSayWindowTicks > 0 ||
		s.RateLimits.MarketSayMax > 0 ||
		s.RateLimits.WhisperWindowTicks > 0 ||
		s.RateLimits.WhisperMax > 0 ||
		s.RateLimits.OfferTradeWindowTicks > 0 ||
		s.RateLimits.OfferTradeMax > 0 ||
		s.RateLimits.PostBoardWindowTicks > 0 ||
		s.RateLimits.PostBoardMax > 0 {
		w.cfg.RateLimits = RateLimitConfig{
			SayWindowTicks:        s.RateLimits.SayWindowTicks,
			SayMax:                s.RateLimits.SayMax,
			MarketSayWindowTicks:  s.RateLimits.MarketSayWindowTicks,
			MarketSayMax:          s.RateLimits.MarketSayMax,
			WhisperWindowTicks:    s.RateLimits.WhisperWindowTicks,
			WhisperMax:            s.RateLimits.WhisperMax,
			OfferTradeWindowTicks: s.RateLimits.OfferTradeWindowTicks,
			OfferTradeMax:         s.RateLimits.OfferTradeMax,
			PostBoardWindowTicks:  s.RateLimits.PostBoardWindowTicks,
			PostBoardMax:          s.RateLimits.PostBoardMax,
		}
	}
	if s.LawNoticeTicks > 0 {
		w.cfg.LawNoticeTicks = s.LawNoticeTicks
	}
	if s.LawVoteTicks > 0 {
		w.cfg.LawVoteTicks = s.LawVoteTicks
	}
	if s.BlueprintAutoPullRange > 0 {
		w.cfg.BlueprintAutoPullRange = s.BlueprintAutoPullRange
	}
	if s.BlueprintBlocksPerTick > 0 {
		w.cfg.BlueprintBlocksPerTick = s.BlueprintBlocksPerTick
	}
	if s.AccessPassCoreRadius > 0 {
		w.cfg.AccessPassCoreRadius = s.AccessPassCoreRadius
	}
	if len(s.MaintenanceCost) > 0 {
		w.cfg.MaintenanceCost = map[string]int{}
		for item, n := range s.MaintenanceCost {
			if item != "" && n > 0 {
				w.cfg.MaintenanceCost[item] = n
			}
		}
		if len(w.cfg.MaintenanceCost) == 0 {
			w.cfg.MaintenanceCost = nil
		}
	}
	if s.FunDecayWindowTicks > 0 {
		w.cfg.FunDecayWindowTicks = s.FunDecayWindowTicks
	}
	if s.FunDecayBase > 0 {
		w.cfg.FunDecayBase = s.FunDecayBase
	}
	if s.StructureSurvivalTicks > 0 {
		w.cfg.StructureSurvivalTicks = s.StructureSurvivalTicks
	}
	w.cfg.applyDefaults()
}

func (w *World) applySnapshotRuntimeState(s snapshot.SnapshotV1) {
	w.weather = s.Weather
	w.weatherUntilTick = s.WeatherUntilTick
	w.activeEventID = s.ActiveEventID
	w.activeEventStart = s.ActiveEventStart
	w.activeEventEnds = s.ActiveEventEnds
	w.activeEventCenter = Vec3i{
		X: s.ActiveEventCenter[0],
		Y: s.ActiveEventCenter[1],
		Z: s.ActiveEventCenter[2],
	}
	w.activeEventRadius = s.ActiveEventRadius
}

