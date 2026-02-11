package world

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"voxelcraft.ai/internal/protocol"
)

func (w *World) joinAgent(name string, delta bool, out chan []byte) JoinResponse {
	if name == "" {
		name = "agent"
	}
	nowTick := w.tick.Load()

	idNum := w.nextAgentNum.Add(1)
	agentID := fmt.Sprintf("A%d", idNum)

	// Spawn near origin on surface.
	spawnXZ := int(idNum) * 2
	spawnX := spawnXZ
	spawnZ := -spawnXZ
	spawnX, spawnZ = w.findSpawnAir(spawnX, spawnZ, 8)

	a := &Agent{
		ID:   agentID,
		Name: name,
		Pos:  Vec3i{X: spawnX, Y: 0, Z: spawnZ},
		Yaw:  0,
	}
	a.initDefaults()
	a.CurrentWorldID = w.cfg.ID
	// Starter items (operational config).
	if w.cfg.StarterItems != nil {
		keys := make([]string, 0, len(w.cfg.StarterItems))
		for item := range w.cfg.StarterItems {
			keys = append(keys, item)
		}
		sort.Strings(keys)
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

	token := fmt.Sprintf("resume_%s_%d", w.cfg.ID, time.Now().UnixNano())
	a.ResumeToken = token

	welcome := protocol.WelcomeMsg{
		Type:            protocol.TypeWelcome,
		ProtocolVersion: protocol.Version,
		AgentID:         agentID,
		ResumeToken:     token,
		CurrentWorldID:  w.cfg.ID,
		WorldManifest: []protocol.WorldRef{
			{
				WorldID:          w.cfg.ID,
				WorldType:        w.cfg.WorldType,
				EntryPointID:     "spawn",
				SwitchCooldown:   w.cfg.SwitchCooldownTicks,
				ResetEveryTicks:  w.cfg.ResetEveryTicks,
				ResetNoticeTicks: w.cfg.ResetNoticeTicks,
			},
		},
		WorldParams: protocol.WorldParams{
			TickRateHz: w.cfg.TickRateHz,
			ChunkSize:  [3]int{16, 16, 1},
			Height:     1,
			ObsRadius:  w.cfg.ObsRadius,
			DayTicks:   w.cfg.DayTicks,
			Seed:       w.cfg.Seed,
		},
		Catalogs: protocol.CatalogDigests{
			BlockPalette:       protocol.DigestRef{Digest: w.catalogs.Blocks.PaletteDigest, Count: len(w.catalogs.Blocks.Palette)},
			ItemPalette:        protocol.DigestRef{Digest: w.catalogs.Items.PaletteDigest, Count: len(w.catalogs.Items.Palette)},
			RecipesDigest:      w.catalogs.Recipes.Digest,
			BlueprintsDigest:   w.catalogs.Blueprints.Digest,
			LawTemplatesDigest: w.catalogs.Laws.Digest,
			EventsDigest:       w.catalogs.Events.Digest,
		},
	}

	tuningCat := w.tuningCatalogMsg()
	welcome.Catalogs.TuningDigest = tuningCat.Digest

	recipesCat := w.recipesCatalogMsg()
	blueprintsCat := w.blueprintsCatalogMsg()
	lawsCat := w.lawTemplatesCatalogMsg()
	eventsCat := w.eventsCatalogMsg()

	catalogMsgs := []protocol.CatalogMsg{
		{
			Type:            protocol.TypeCatalog,
			ProtocolVersion: protocol.Version,
			Name:            "block_palette",
			Digest:          w.catalogs.Blocks.PaletteDigest,
			Part:            1,
			TotalParts:      1,
			Data:            w.catalogs.Blocks.Palette,
		},
		{
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
	}

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
	agentIDs := make([]string, 0, len(w.agents))
	for id := range w.agents {
		agentIDs = append(agentIDs, id)
	}
	sort.Strings(agentIDs)
	var a *Agent
	for _, id := range agentIDs {
		aa := w.agents[id]
		if aa != nil && aa.ResumeToken == token {
			a = aa
			break
		}
	}
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
	newToken := fmt.Sprintf("resume_%s_%d", w.cfg.ID, time.Now().UnixNano())
	a.ResumeToken = newToken

	// If a world event is active, inform the resuming agent.
	w.enqueueActiveEventForAgent(w.tick.Load(), a)

	welcome := protocol.WelcomeMsg{
		Type:            protocol.TypeWelcome,
		ProtocolVersion: protocol.Version,
		AgentID:         a.ID,
		ResumeToken:     newToken,
		CurrentWorldID:  w.cfg.ID,
		WorldManifest: []protocol.WorldRef{
			{
				WorldID:          w.cfg.ID,
				WorldType:        w.cfg.WorldType,
				EntryPointID:     "spawn",
				SwitchCooldown:   w.cfg.SwitchCooldownTicks,
				ResetEveryTicks:  w.cfg.ResetEveryTicks,
				ResetNoticeTicks: w.cfg.ResetNoticeTicks,
			},
		},
		WorldParams: protocol.WorldParams{
			TickRateHz: w.cfg.TickRateHz,
			ChunkSize:  [3]int{16, 16, 1},
			Height:     1,
			ObsRadius:  w.cfg.ObsRadius,
			DayTicks:   w.cfg.DayTicks,
			Seed:       w.cfg.Seed,
		},
		Catalogs: protocol.CatalogDigests{
			BlockPalette:       protocol.DigestRef{Digest: w.catalogs.Blocks.PaletteDigest, Count: len(w.catalogs.Blocks.Palette)},
			ItemPalette:        protocol.DigestRef{Digest: w.catalogs.Items.PaletteDigest, Count: len(w.catalogs.Items.Palette)},
			RecipesDigest:      w.catalogs.Recipes.Digest,
			BlueprintsDigest:   w.catalogs.Blueprints.Digest,
			LawTemplatesDigest: w.catalogs.Laws.Digest,
			EventsDigest:       w.catalogs.Events.Digest,
		},
	}

	tuningCat := w.tuningCatalogMsg()
	welcome.Catalogs.TuningDigest = tuningCat.Digest

	recipesCat := w.recipesCatalogMsg()
	blueprintsCat := w.blueprintsCatalogMsg()
	lawsCat := w.lawTemplatesCatalogMsg()
	eventsCat := w.eventsCatalogMsg()

	catalogMsgs := []protocol.CatalogMsg{
		{
			Type:            protocol.TypeCatalog,
			ProtocolVersion: protocol.Version,
			Name:            "block_palette",
			Digest:          w.catalogs.Blocks.PaletteDigest,
			Part:            1,
			TotalParts:      1,
			Data:            w.catalogs.Blocks.Palette,
		},
		{
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
	}

	if req.Resp != nil {
		req.Resp <- JoinResponse{Welcome: welcome, Catalogs: catalogMsgs}
	}
}
