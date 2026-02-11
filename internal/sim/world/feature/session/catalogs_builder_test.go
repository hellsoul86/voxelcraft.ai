package session

import (
	"testing"

	"voxelcraft.ai/internal/sim/catalogs"
)

func TestBuildWelcome(t *testing.T) {
	msg := BuildWelcome(WelcomeInput{
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

func TestCatalogBuildersSorted(t *testing.T) {
	recipes := map[string]catalogs.RecipeDef{
		"b": {RecipeID: "b"},
		"a": {RecipeID: "a"},
	}
	rc := RecipesCatalogMsg("r", recipes)
	defs, ok := rc.Data.([]catalogs.RecipeDef)
	if !ok || len(defs) != 2 {
		t.Fatalf("unexpected recipes data: %T %#v", rc.Data, rc.Data)
	}
	if defs[0].RecipeID != "a" || defs[1].RecipeID != "b" {
		t.Fatalf("recipes not sorted: %#v", defs)
	}

	blueprints := map[string]catalogs.BlueprintDef{
		"z": {ID: "z"},
		"c": {ID: "c"},
	}
	bc := BlueprintsCatalogMsg("b", blueprints)
	bdefs, ok := bc.Data.([]catalogs.BlueprintDef)
	if !ok || len(bdefs) != 2 {
		t.Fatalf("unexpected blueprints data: %T %#v", bc.Data, bc.Data)
	}
	if bdefs[0].ID != "c" || bdefs[1].ID != "z" {
		t.Fatalf("blueprints not sorted: %#v", bdefs)
	}

	tpls := []catalogs.LawTemplate{{ID: "z"}, {ID: "a"}}
	lc := LawTemplatesCatalogMsg("l", tpls)
	ldata, ok := lc.Data.(LawTemplatesCatalogData)
	if !ok || len(ldata.Templates) != 2 {
		t.Fatalf("unexpected laws data: %T %#v", lc.Data, lc.Data)
	}
	if ldata.Templates[0].ID != "a" || ldata.Templates[1].ID != "z" {
		t.Fatalf("laws not sorted: %#v", ldata.Templates)
	}
}
