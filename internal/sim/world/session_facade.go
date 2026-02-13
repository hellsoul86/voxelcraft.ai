package world

import (
	"strings"
	"time"

	"voxelcraft.ai/internal/protocol"
	catalogspkg "voxelcraft.ai/internal/sim/world/feature/session/catalogs"
	lifecyclepkg "voxelcraft.ai/internal/sim/world/feature/session/lifecycle"
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
	return catalogspkg.BuildHandshakeCatalogs(catalogspkg.BuildHandshakeCatalogsInput{
		BlockPalette:       w.catalogs.Blocks.Palette,
		BlockPaletteDigest: w.catalogs.Blocks.PaletteDigest,
		ItemPalette:        w.catalogs.Items.Palette,
		ItemPaletteDigest:  w.catalogs.Items.PaletteDigest,
		RecipesDigest:      w.catalogs.Recipes.Digest,
		RecipesByID:        w.catalogs.Recipes.ByID,
		BlueprintsDigest:   w.catalogs.Blueprints.Digest,
		BlueprintsByID:     w.catalogs.Blueprints.ByID,
		LawsDigest:         w.catalogs.Laws.Digest,
		LawTemplates:       w.catalogs.Laws.Templates,
		EventsDigest:       w.catalogs.Events.Digest,
		EventsByID:         w.catalogs.Events.ByID,
		Tuning: catalogspkg.TuningInput{
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
		},
	})
}

func (w *World) joinAgent(name string, delta bool, out chan []byte) JoinResponse {
	name = catalogspkg.NormalizeAgentName(name)
	nowTick := w.tick.Load()

	idNum := w.nextAgentNum.Add(1)
	agentID := lifecyclepkg.NewAgentID(idNum)

	// Spawn near origin on surface.
	spawnX, spawnZ := lifecyclepkg.SpawnSeed(idNum)
	spawnX, spawnZ = w.findSpawnAir(spawnX, spawnZ, 8)

	a := lifecyclepkg.BuildJoinedAgent(lifecyclepkg.BuildJoinedAgentInput{
		AgentID:      agentID,
		Name:         name,
		WorldID:      w.cfg.ID,
		Spawn:        Vec3i{X: spawnX, Y: 0, Z: spawnZ},
		StarterItems: w.cfg.StarterItems,
	})

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

	attached := lifecyclepkg.AttachByToken(token, w.agents, w.cfg.ID, time.Now().UnixNano())
	if !attached.OK || attached.Agent == nil {
		if req.Resp != nil {
			req.Resp <- JoinResponse{}
		}
		return
	}
	a := attached.Agent

	// Attach client (does not affect simulation determinism).
	w.clients[a.ID] = &clientState{Out: req.Out, DeltaVoxels: req.DeltaVoxels}

	// If a world event is active, inform the resuming agent.
	w.enqueueActiveEventForAgent(w.tick.Load(), a)

	welcome := w.buildWelcome(a.ID, attached.NewToken)
	catalogMsgs, tuningDigest := w.buildCatalogMsgs()
	welcome.Catalogs.TuningDigest = tuningDigest

	if req.Resp != nil {
		req.Resp <- JoinResponse{Welcome: welcome, Catalogs: catalogMsgs}
	}
}
