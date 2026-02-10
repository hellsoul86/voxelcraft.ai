package world

import (
	"sort"

	"voxelcraft.ai/internal/protocol"
)

func (w *World) maybeSeasonRollover(nowTick uint64) {
	seasonLen := uint64(w.cfg.SeasonLengthTicks)
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
	seasonLen := uint64(w.cfg.SeasonLengthTicks)
	if seasonLen == 0 {
		return 1
	}
	return int(nowTick/seasonLen) + 1
}

func (w *World) seasonDay(nowTick uint64) int {
	dayTicks := uint64(w.cfg.DayTicks)
	if dayTicks == 0 {
		return 1
	}
	seasonLen := uint64(w.cfg.SeasonLengthTicks)
	seasonDays := seasonLen / dayTicks
	if seasonDays == 0 {
		seasonDays = 1
	}
	return int((nowTick/dayTicks)%seasonDays) + 1
}

func (w *World) resetWorldForNewSeason(nowTick uint64, newSeason int, archiveTick uint64) {
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
			o.Treasury = map[string]int{}
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
	}
}

func (w *World) resetAgentForNewSeason(nowTick uint64, a *Agent) {
	// Cancel ongoing tasks.
	a.MoveTask = nil
	a.WorkTask = nil

	// Reset physical attributes.
	a.HP = 20
	a.Hunger = 20
	a.StaminaMilli = 1000
	a.Yaw = 0

	// Reset inventory to starter kit; preserve memory and reputation.
	a.Inventory = map[string]int{
		"PLANK":   20,
		"COAL":    10,
		"STONE":   20,
		"BERRIES": 10,
	}

	// Reset equipment (MVP).
	a.Equipment = Equipment{MainHand: "NONE", Armor: [4]string{"NONE", "NONE", "NONE", "NONE"}}

	// Clear ephemeral queues.
	a.Events = nil
	a.PendingMemory = nil

	// Reset anti-exploit windows so novelty/fun can be earned per season.
	a.rl = map[string]*rateWindow{}
	a.funDecay = map[string]*funDecayWindow{}
	a.seenBiomes = map[string]bool{}
	a.seenRecipes = map[string]bool{}
	a.seenEvents = map[string]bool{}
	a.Fun = FunScore{}

	// Respawn at deterministic spawn point (depends on agent number) on new terrain.
	n := agentNum(a.ID)
	spawnXZ := n * 2
	spawnX := spawnXZ
	spawnZ := -spawnXZ
	y := w.surfaceY(spawnX, spawnZ)
	a.Pos = Vec3i{X: spawnX, Y: y, Z: spawnZ}

	// Award novelty for the first biome arrival in the season.
	w.funOnBiome(a, nowTick)
}
