package welcome

import "voxelcraft.ai/internal/protocol"

type Input struct {
	AgentID            string
	ResumeToken        string
	CurrentWorld       string
	WorldType          string
	SwitchCooldown     int
	ResetEveryTicks    int
	ResetNoticeTicks   int
	TickRateHz         int
	ObsRadius          int
	DayTicks           int
	Seed               int64
	BlockPaletteDigest string
	BlockPaletteCount  int
	ItemPaletteDigest  string
	ItemPaletteCount   int
	RecipesDigest      string
	BlueprintsDigest   string
	LawTemplatesDigest string
	EventsDigest       string
}

func Build(in Input) protocol.WelcomeMsg {
	return protocol.WelcomeMsg{
		Type:            protocol.TypeWelcome,
		ProtocolVersion: protocol.Version,
		AgentID:         in.AgentID,
		ResumeToken:     in.ResumeToken,
		CurrentWorldID:  in.CurrentWorld,
		WorldManifest: []protocol.WorldRef{{
			WorldID:          in.CurrentWorld,
			WorldType:        in.WorldType,
			EntryPointID:     "spawn",
			SwitchCooldown:   in.SwitchCooldown,
			ResetEveryTicks:  in.ResetEveryTicks,
			ResetNoticeTicks: in.ResetNoticeTicks,
		}},
		WorldParams: protocol.WorldParams{
			TickRateHz: in.TickRateHz,
			ChunkSize:  [3]int{16, 16, 1},
			Height:     1,
			ObsRadius:  in.ObsRadius,
			DayTicks:   in.DayTicks,
			Seed:       in.Seed,
		},
		Catalogs: protocol.CatalogDigests{
			BlockPalette:       protocol.DigestRef{Digest: in.BlockPaletteDigest, Count: in.BlockPaletteCount},
			ItemPalette:        protocol.DigestRef{Digest: in.ItemPaletteDigest, Count: in.ItemPaletteCount},
			RecipesDigest:      in.RecipesDigest,
			BlueprintsDigest:   in.BlueprintsDigest,
			LawTemplatesDigest: in.LawTemplatesDigest,
			EventsDigest:       in.EventsDigest,
		},
	}
}
