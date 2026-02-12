package welcome

import "testing"

func TestBuildWelcome(t *testing.T) {
	msg := Build(Input{
		AgentID:            "A1",
		ResumeToken:        "r1",
		CurrentWorld:       "OVERWORLD",
		WorldType:          "OVERWORLD",
		SwitchCooldown:     50,
		ResetEveryTicks:    3000,
		ResetNoticeTicks:   300,
		TickRateHz:         5,
		ObsRadius:          7,
		DayTicks:           6000,
		Seed:               123,
		BlockPaletteDigest: "bd",
		BlockPaletteCount:  10,
		ItemPaletteDigest:  "id",
		ItemPaletteCount:   20,
		RecipesDigest:      "rd",
		BlueprintsDigest:   "bpd",
		LawTemplatesDigest: "ld",
		EventsDigest:       "ed",
	})
	if msg.AgentID != "A1" || msg.ResumeToken != "r1" {
		t.Fatalf("unexpected welcome identity: %+v", msg)
	}
	if msg.WorldParams.Height != 1 || msg.WorldParams.ChunkSize != [3]int{16, 16, 1} {
		t.Fatalf("unexpected world params: %+v", msg.WorldParams)
	}
	if msg.Catalogs.RecipesDigest != "rd" || msg.Catalogs.EventsDigest != "ed" {
		t.Fatalf("unexpected catalog digests: %+v", msg.Catalogs)
	}
}
