package world

import (
	"strings"
	"time"

	"voxelcraft.ai/internal/protocol"
	catalogspkg "voxelcraft.ai/internal/sim/world/feature/session/catalogs"
	lifecyclepkg "voxelcraft.ai/internal/sim/world/feature/session/lifecycle"
	resumepkg "voxelcraft.ai/internal/sim/world/feature/session/resume"
	welcomepkg "voxelcraft.ai/internal/sim/world/feature/session/welcome"
)

func (w *World) buildWelcome(agentID, resumeToken string) protocol.WelcomeMsg {
	return welcomepkg.Build(welcomepkg.Input{
		AgentID:            agentID,
		ResumeToken:        resumeToken,
		CurrentWorld:       w.cfg.ID,
		WorldType:          w.cfg.WorldType,
		SwitchCooldown:     w.cfg.SwitchCooldownTicks,
		ResetEveryTicks:    w.cfg.ResetEveryTicks,
		ResetNoticeTicks:   w.cfg.ResetNoticeTicks,
		TickRateHz:         w.cfg.TickRateHz,
		ObsRadius:          w.cfg.ObsRadius,
		DayTicks:           w.cfg.DayTicks,
		Seed:               w.cfg.Seed,
		BlockPaletteDigest: w.catalogs.Blocks.PaletteDigest,
		BlockPaletteCount:  len(w.catalogs.Blocks.Palette),
		ItemPaletteDigest:  w.catalogs.Items.PaletteDigest,
		ItemPaletteCount:   len(w.catalogs.Items.Palette),
		RecipesDigest:      w.catalogs.Recipes.Digest,
		BlueprintsDigest:   w.catalogs.Blueprints.Digest,
		LawTemplatesDigest: w.catalogs.Laws.Digest,
		EventsDigest:       w.catalogs.Events.Digest,
	})
}

func (w *World) buildCatalogMsgs() ([]protocol.CatalogMsg, string) {
	tuningCat := catalogspkg.TuningCatalogMsg(catalogspkg.TuningInput{
		SnapshotEveryTicks: w.cfg.SnapshotEveryTicks,
		DirectorEveryTicks: w.cfg.DirectorEveryTicks,
		SeasonLengthTicks:  w.cfg.SeasonLengthTicks,
		RateLimits: catalogspkg.TuningRateLimits{
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
		MaintenanceCost:        w.cfg.MaintenanceCost,
		FunDecayWindowTicks:    w.cfg.FunDecayWindowTicks,
		FunDecayBase:           w.cfg.FunDecayBase,
		StructureSurvivalTicks: w.cfg.StructureSurvivalTicks,
	})
	recipesCat := catalogspkg.RecipesCatalogMsg(w.catalogs.Recipes.Digest, w.catalogs.Recipes.ByID)
	blueprintsCat := catalogspkg.BlueprintsCatalogMsg(w.catalogs.Blueprints.Digest, w.catalogs.Blueprints.ByID)
	lawsCat := catalogspkg.LawTemplatesCatalogMsg(w.catalogs.Laws.Digest, w.catalogs.Laws.Templates)
	eventsCat := catalogspkg.EventsCatalogMsg(w.catalogs.Events.Digest, w.catalogs.Events.ByID)

	catalogMsgs := catalogspkg.OrderedCatalogs(
		protocol.CatalogMsg{
			Type:            protocol.TypeCatalog,
			ProtocolVersion: protocol.Version,
			Name:            "block_palette",
			Digest:          w.catalogs.Blocks.PaletteDigest,
			Part:            1,
			TotalParts:      1,
			Data:            w.catalogs.Blocks.Palette,
		},
		protocol.CatalogMsg{
			Type:            protocol.TypeCatalog,
			ProtocolVersion: protocol.Version,
			Name:            "item_palette",
			Digest:          w.catalogs.Items.PaletteDigest,
			Part:            1,
			TotalParts:      1,
			Data:            w.catalogs.Items.Palette,
		},
		tuningCat,
		recipesCat,
		blueprintsCat,
		lawsCat,
		eventsCat,
	)
	return catalogMsgs, tuningCat.Digest
}

func (w *World) joinAgent(name string, delta bool, out chan []byte) JoinResponse {
	name = catalogspkg.NormalizeAgentName(name)
	nowTick := w.tick.Load()

	idNum := w.nextAgentNum.Add(1)
	agentID := lifecyclepkg.NewAgentID(idNum)

	// Spawn near origin on surface.
	spawnX, spawnZ := lifecyclepkg.SpawnSeed(idNum)
	spawnX, spawnZ = w.findSpawnAir(spawnX, spawnZ, 8)

	a := &Agent{
		ID:   agentID,
		Name: name,
		Pos:  Vec3i{X: spawnX, Y: 0, Z: spawnZ},
		Yaw:  0,
	}
	a.InitDefaults()
	a.CurrentWorldID = w.cfg.ID
	// Starter items (operational config).
	if w.cfg.StarterItems != nil {
		keys := resumepkg.SortedIDs(w.cfg.StarterItems)
		for _, item := range keys {
			n := w.cfg.StarterItems[item]
			if item == "" || n <= 0 {
				continue
			}
			a.Inventory[item] += n
		}
	}

	// Fun/novelty: first biome arrival.
	w.funOnBiome(a, nowTick)

	// If a world event is active, inform the joining agent immediately.
	w.enqueueActiveEventForAgent(nowTick, a)

	w.agents[agentID] = a
	if out != nil {
		w.clients[agentID] = &clientState{Out: out, DeltaVoxels: delta}
	}

	token := lifecyclepkg.NewResumeToken(w.cfg.ID, time.Now().UnixNano())
	a.ResumeToken = token

	welcome := w.buildWelcome(agentID, token)
	catalogMsgs, tuningDigest := w.buildCatalogMsgs()
	welcome.Catalogs.TuningDigest = tuningDigest

	return JoinResponse{Welcome: welcome, Catalogs: catalogMsgs}
}

func (w *World) handleJoin(req JoinRequest) {
	resp := w.joinAgent(req.Name, req.DeltaVoxels, req.Out)
	if req.Resp != nil {
		req.Resp <- resp
	}
}

func (w *World) handleAttach(req AttachRequest) {
	token := strings.TrimSpace(req.ResumeToken)
	if token == "" || req.Out == nil {
		if req.Resp != nil {
			req.Resp <- JoinResponse{}
		}
		return
	}

	// Find agent deterministically by iterating sorted ids.
	candidates := make([]resumepkg.Candidate, 0, len(w.agents))
	for id, aa := range w.agents {
		if aa == nil {
			continue
		}
		candidates = append(candidates, resumepkg.Candidate{ID: id, ResumeToken: aa.ResumeToken})
	}
	aid := resumepkg.FindResumeAgentID(candidates, token)
	a := w.agents[aid]
	if a == nil {
		if req.Resp != nil {
			req.Resp <- JoinResponse{}
		}
		return
	}
	a.CurrentWorldID = w.cfg.ID

	// Attach client (does not affect simulation determinism).
	w.clients[a.ID] = &clientState{Out: req.Out, DeltaVoxels: req.DeltaVoxels}

	// Rotate token on successful resume.
	newToken := lifecyclepkg.NewResumeToken(w.cfg.ID, time.Now().UnixNano())
	a.ResumeToken = newToken

	// If a world event is active, inform the resuming agent.
	w.enqueueActiveEventForAgent(w.tick.Load(), a)

	welcome := w.buildWelcome(a.ID, newToken)
	catalogMsgs, tuningDigest := w.buildCatalogMsgs()
	welcome.Catalogs.TuningDigest = tuningDigest

	if req.Resp != nil {
		req.Resp <- JoinResponse{Welcome: welcome, Catalogs: catalogMsgs}
	}
}
