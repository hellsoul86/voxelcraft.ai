package world

import (
	"fmt"

	"voxelcraft.ai/internal/persistence/snapshot"
	"voxelcraft.ai/internal/sim/world/logic/ids"
)

// ImportSnapshot replaces the current in-memory world state with the snapshot.
// It sets the world's tick to snapshotTick+1 (the next tick to simulate).
//
// This must be called only when the world is stopped or from the world loop goroutine.
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

	maxAgent, maxTask := w.importSnapshotAgents(s)
	w.nextAgentNum.Store(maxU64(maxAgent, s.Counters.NextAgent))
	w.nextTaskNum.Store(maxU64(maxTask, s.Counters.NextTask))

	maxLand := w.importSnapshotClaims(s)
	w.nextLandNum.Store(maxU64(maxLand, s.Counters.NextLand))

	w.importSnapshotContainers(s)
	maxItem := w.importSnapshotItems(s)
	w.nextItemNum.Store(maxU64(maxItem, s.Counters.NextItem))
	w.importSnapshotSigns(s)
	w.importSnapshotConveyors(s)
	w.importSnapshotSwitches(s)

	maxTrade := w.importSnapshotTrades(s)
	w.nextTradeNum.Store(maxU64(maxTrade, s.Counters.NextTrade))

	maxPost := w.importSnapshotBoards(s)
	w.nextPostNum.Store(maxU64(maxPost, s.Counters.NextPost))

	maxContract := w.importSnapshotContracts(s)
	w.nextContractNum.Store(maxU64(maxContract, s.Counters.NextContract))

	maxLaw := w.importSnapshotLaws(s)
	w.nextLawNum.Store(maxU64(maxLaw, s.Counters.NextLaw))

	maxOrg := w.importSnapshotOrgs(s)
	w.nextOrgNum.Store(maxU64(maxOrg, s.Counters.NextOrg))

	w.importSnapshotStructures(s)
	w.importSnapshotStats(s)

	// Resume on the next tick.
	w.tick.Store(s.Header.Tick + 1)
	return nil
}

func maxU64(a, b uint64) uint64 {
	return ids.MaxU64(a, b)
}

func parseUintAfterPrefix(prefix, id string) (uint64, bool) {
	return ids.ParseUintAfterPrefix(prefix, id)
}

func parseLandNum(id string) (uint64, bool) {
	return ids.ParseLandNum(id)
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
			dirty:  true,
		}
		_ = c.Digest()
		store.chunks[k] = c
	}
	w.chunks = store
	return nil
}

func (w *World) importSnapshotTrades(s snapshot.SnapshotV1) (maxTrade uint64) {
	w.trades = map[string]*Trade{}
	for _, tr := range s.Trades {
		offer := map[string]int{}
		for item, n := range tr.Offer {
			if n > 0 {
				offer[item] = n
			}
		}
		req := map[string]int{}
		for item, n := range tr.Request {
			if n > 0 {
				req[item] = n
			}
		}
		w.trades[tr.TradeID] = &Trade{
			TradeID:     tr.TradeID,
			From:        tr.From,
			To:          tr.To,
			Offer:       offer,
			Request:     req,
			CreatedTick: tr.CreatedTick,
		}
		if n, ok := parseUintAfterPrefix("TR", tr.TradeID); ok && n > maxTrade {
			maxTrade = n
		}
	}
	return maxTrade
}

func (w *World) importSnapshotBoards(s snapshot.SnapshotV1) (maxPost uint64) {
	w.boards = map[string]*Board{}
	for _, b := range s.Boards {
		bb := &Board{BoardID: b.BoardID}
		for _, p := range b.Posts {
			bb.Posts = append(bb.Posts, BoardPost{
				PostID: p.PostID,
				Author: p.Author,
				Title:  p.Title,
				Body:   p.Body,
				Tick:   p.Tick,
			})
			if n, ok := parseUintAfterPrefix("P", p.PostID); ok && n > maxPost {
				maxPost = n
			}
		}
		w.boards[bb.BoardID] = bb
	}
	return maxPost
}
