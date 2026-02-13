package world

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
	runtimepkg "voxelcraft.ai/internal/sim/world/feature/director/runtime"
	respawnpkg "voxelcraft.ai/internal/sim/world/feature/survival/respawn"
)

func (w *World) maybeSeasonRollover(nowTick uint64) {
	seasonLen := runtimepkg.SeasonLengthTicks(w.cfg.ResetEveryTicks, w.cfg.SeasonLengthTicks)
	if seasonLen == 0 || nowTick == 0 || nowTick%seasonLen != 0 {
		return
	}

	endedSeason := int(nowTick / seasonLen)
	newSeason := endedSeason + 1
	archiveTick := nowTick - 1

	// Force an end-of-season snapshot (best-effort, but we block to avoid silently losing archives).
	if w.snapshotSink != nil {
		snap := w.ExportSnapshot(archiveTick)
		w.snapshotSink <- snap
	}

	// Reset world state for the new season (keep cultural assets like org metadata).
	w.resetWorldForNewSeason(nowTick, newSeason, archiveTick)
}

func (w *World) seasonIndex(nowTick uint64) int {
	return runtimepkg.SeasonIndex(nowTick, w.cfg.ResetEveryTicks, w.cfg.SeasonLengthTicks)
}

func (w *World) seasonDay(nowTick uint64) int {
	return runtimepkg.SeasonDay(nowTick, w.cfg.DayTicks, w.cfg.ResetEveryTicks, w.cfg.SeasonLengthTicks)
}

func (w *World) maybeWorldResetNotice(nowTick uint64) {
	ok, resetTick := runtimepkg.ShouldWorldResetNotice(nowTick, w.cfg.ResetEveryTicks, w.cfg.ResetNoticeTicks)
	if !ok {
		return
	}
	notice := uint64(w.cfg.ResetNoticeTicks)
	for _, a := range w.agents {
		a.AddEvent(protocol.Event{
			"t":          nowTick,
			"type":       "WORLD_RESET_NOTICE",
			"world_id":   w.cfg.ID,
			"reset_tick": resetTick,
			"in_ticks":   notice,
		})
	}
}

func (w *World) resetWorldForNewSeason(nowTick uint64, newSeason int, archiveTick uint64) {
	w.resetTotal++

	// Advance world seed to reshuffle resources deterministically.
	w.cfg.Seed++

	// Reset terrain/chunks with the new seed.
	gen := w.chunks.gen
	gen.Seed = w.cfg.Seed
	w.chunks = NewChunkStore(gen)

	// Reset world-scoped mutable state.
	w.weather = "CLEAR"
	w.weatherUntilTick = 0
	w.activeEventID = ""
	w.activeEventStart = 0
	w.activeEventEnds = 0
	w.activeEventCenter = Vec3i{}
	w.activeEventRadius = 0

	w.claims = map[string]*LandClaim{}
	w.containers = map[Vec3i]*Container{}
	w.items = map[string]*ItemEntity{}
	w.itemsAt = map[Vec3i][]string{}
	w.trades = map[string]*Trade{}
	w.boards = map[string]*Board{}
	w.signs = map[Vec3i]*Sign{}
	w.conveyors = map[Vec3i]ConveyorMeta{}
	w.switches = map[Vec3i]bool{}
	w.contracts = map[string]*Contract{}
	w.laws = map[string]*Law{}
	w.structures = map[string]*Structure{}
	w.stats = NewWorldStats(300, 72000)

	// Organizations are treated as cultural assets: keep their identity and membership, but
	// reset treasuries to avoid carrying physical wealth across seasons.
	if len(w.orgs) > 0 {
		orgIDs := make([]string, 0, len(w.orgs))
		for id := range w.orgs {
			orgIDs = append(orgIDs, id)
		}
		sort.Strings(orgIDs)
		for _, id := range orgIDs {
			o := w.orgs[id]
			if o == nil {
				continue
			}
			if o.TreasuryByWorld == nil {
				o.TreasuryByWorld = map[string]map[string]int{}
			}
			o.TreasuryByWorld[w.cfg.ID] = map[string]int{}
			o.Treasury = o.TreasuryByWorld[w.cfg.ID]
		}
	}

	// Reset agent physical state; keep identity, org membership, reputation, and memory.
	agents := w.sortedAgents()
	for _, a := range agents {
		if a == nil {
			continue
		}
		w.resetAgentForNewSeason(nowTick, a)
		a.AddEvent(protocol.Event{
			"t":            nowTick,
			"type":         "SEASON_ROLLOVER",
			"season":       newSeason,
			"archive_tick": archiveTick,
			"seed":         w.cfg.Seed,
		})
		a.AddEvent(protocol.Event{
			"t":          nowTick,
			"type":       "WORLD_RESET_DONE",
			"world_id":   w.cfg.ID,
			"reset_tick": nowTick,
		})
	}
	w.auditEvent(nowTick, "SYSTEM", "WORLD_RESET", Vec3i{}, "SEASON_ROLLOVER", map[string]any{
		"world_id":     w.cfg.ID,
		"archive_tick": archiveTick,
		"season":       newSeason,
		"new_seed":     w.cfg.Seed,
	})
}

func (w *World) resetAgentForNewSeason(nowTick uint64, a *Agent) {
	respawnpkg.ResetForSeason(a, w.findSpawnAir)
	// Award novelty for the first biome arrival in the season.
	w.funOnBiome(a, nowTick)
}
