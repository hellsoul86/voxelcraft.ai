package world

import (
	"sort"

	"voxelcraft.ai/internal/persistence/snapshot"
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
	agentSnaps := w.exportSnapshotAgents(nowTick)
	claimSnaps := w.exportSnapshotClaims()
	containerSnaps := w.exportSnapshotContainers()
	itemSnaps := w.exportSnapshotItems(nowTick)
	signSnaps := w.exportSnapshotSigns()
	conveyorSnaps := w.exportSnapshotConveyors()
	switchSnaps := w.exportSnapshotSwitches()
	tradeSnaps := w.exportSnapshotTrades()
	boardSnaps := w.exportSnapshotBoards()
	contractSnaps := w.exportSnapshotContracts()
	lawSnaps := w.exportSnapshotLaws()
	orgSnaps := w.exportSnapshotOrgs()
	structSnaps := w.exportSnapshotStructures()
	statsSnap := w.exportSnapshotStats()
	maintCost := w.exportSnapshotMaintenanceCost()
	starterItems := w.exportSnapshotStarterItems()

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
		StarterItems:                    starterItems,
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
		MaintenanceCost:        maintCost,
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
		Agents:                 agentSnaps,
		Claims:                 claimSnaps,
		Containers:             containerSnaps,
		Items:                  itemSnaps,
		Signs:                  signSnaps,
		Conveyors:              conveyorSnaps,
		Switches:               switchSnaps,
		Trades:                 tradeSnaps,
		Boards:                 boardSnaps,
		Contracts:              contractSnaps,
		Laws:                   lawSnaps,
		Orgs:                   orgSnaps,
		Structures:             structSnaps,
		Stats:                  statsSnap,
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

func (w *World) exportSnapshotStats() *snapshot.StatsV1 {
	if w.stats == nil {
		return nil
	}
	out := &snapshot.StatsV1{
		BucketTicks: w.stats.BucketTicks,
		WindowTicks: w.stats.WindowTicksV,
		CurIdx:      w.stats.CurIdx,
		CurBase:     w.stats.CurBase,
		Buckets:     make([]snapshot.StatsBucketV1, len(w.stats.Buckets)),
	}
	for i, b := range w.stats.Buckets {
		out.Buckets[i] = snapshot.StatsBucketV1{
			Trades:             b.Trades,
			Denied:             b.Denied,
			ChunksDiscovered:   b.ChunksDiscovered,
			BlueprintsComplete: b.BlueprintsComplete,
		}
	}
	if len(w.stats.SeenChunks) > 0 {
		keys := make([]StatsChunkKey, 0, len(w.stats.SeenChunks))
		for k := range w.stats.SeenChunks {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].CX != keys[j].CX {
				return keys[i].CX < keys[j].CX
			}
			return keys[i].CZ < keys[j].CZ
		})
		out.SeenChunks = make([]snapshot.ChunkKeyV1, 0, len(keys))
		for _, k := range keys {
			out.SeenChunks = append(out.SeenChunks, snapshot.ChunkKeyV1{CX: k.CX, CZ: k.CZ})
		}
	}
	return out
}

func (w *World) exportSnapshotMaintenanceCost() map[string]int {
	return snapshotcodec.PositiveMap(w.cfg.MaintenanceCost)
}

func (w *World) exportSnapshotStarterItems() map[string]int {
	return snapshotcodec.PositiveMap(w.cfg.StarterItems)
}

func (w *World) exportSnapshotTrades() []snapshot.TradeV1 {
	tradeIDs := make([]string, 0, len(w.trades))
	for id := range w.trades {
		tradeIDs = append(tradeIDs, id)
	}
	sort.Strings(tradeIDs)
	tradeSnaps := make([]snapshot.TradeV1, 0, len(tradeIDs))
	for _, id := range tradeIDs {
		tr := w.trades[id]
		offer := map[string]int{}
		for k, v := range tr.Offer {
			if v != 0 {
				offer[k] = v
			}
		}
		req := map[string]int{}
		for k, v := range tr.Request {
			if v != 0 {
				req[k] = v
			}
		}
		tradeSnaps = append(tradeSnaps, snapshot.TradeV1{
			TradeID:     tr.TradeID,
			From:        tr.From,
			To:          tr.To,
			Offer:       offer,
			Request:     req,
			CreatedTick: tr.CreatedTick,
		})
	}
	return tradeSnaps
}

func (w *World) exportSnapshotBoards() []snapshot.BoardV1 {
	boardIDs := make([]string, 0, len(w.boards))
	for id := range w.boards {
		boardIDs = append(boardIDs, id)
	}
	sort.Strings(boardIDs)
	boardSnaps := make([]snapshot.BoardV1, 0, len(boardIDs))
	for _, id := range boardIDs {
		b := w.boards[id]
		if b == nil {
			continue
		}
		posts := make([]snapshot.BoardPostV1, 0, len(b.Posts))
		for _, p := range b.Posts {
			posts = append(posts, snapshot.BoardPostV1{
				PostID: p.PostID,
				Author: p.Author,
				Title:  p.Title,
				Body:   p.Body,
				Tick:   p.Tick,
			})
		}
		boardSnaps = append(boardSnaps, snapshot.BoardV1{BoardID: id, Posts: posts})
	}
	return boardSnaps
}
